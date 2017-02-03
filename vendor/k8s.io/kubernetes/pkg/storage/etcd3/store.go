/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd3

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	"golang.org/x/net/context"
)

type store struct {
	client *clientv3.Client
	// getOpts contains additional options that should be passed
	// to all Get() calls.
	getOps     []clientv3.OpOption
	codec      runtime.Codec
	versioner  storage.Versioner
	pathPrefix string
	watcher    *watcher
}

type elemForDecode struct {
	data []byte
	rev  uint64
}

type objState struct {
	obj  runtime.Object
	meta *storage.ResponseMeta
	rev  int64
	data []byte
}

// New returns an etcd3 implementation of storage.Interface.
func New(c *clientv3.Client, codec runtime.Codec, prefix string) storage.Interface {
	return newStore(c, true, codec, prefix)
}

// NewWithNoQuorumRead returns etcd3 implementation of storage.Interface
// where Get operations don't require quorum read.
func NewWithNoQuorumRead(c *clientv3.Client, codec runtime.Codec, prefix string) storage.Interface {
	return newStore(c, false, codec, prefix)
}

func newStore(c *clientv3.Client, quorumRead bool, codec runtime.Codec, prefix string) *store {
	versioner := etcd.APIObjectVersioner{}
	result := &store{
		client:     c,
		versioner:  versioner,
		codec:      codec,
		pathPrefix: prefix,
		watcher:    newWatcher(c, codec, versioner),
	}
	if !quorumRead {
		// In case of non-quorum reads, we can set WithSerializable()
		// options for all Get operations.
		result.getOps = append(result.getOps, clientv3.WithSerializable())
	}
	return result
}

// Versioner implements storage.Interface.Versioner.
func (s *store) Versioner() storage.Versioner {
	return s.versioner
}

// Get implements storage.Interface.Get.
func (s *store) Get(ctx context.Context, key string, out runtime.Object, ignoreNotFound bool) error {
	key = keyWithPrefix(s.pathPrefix, key)
	getResp, err := s.client.KV.Get(ctx, key, s.getOps...)
	if err != nil {
		return err
	}

	if len(getResp.Kvs) == 0 {
		if ignoreNotFound {
			return runtime.SetZeroValue(out)
		}
		return storage.NewKeyNotFoundError(key, 0)
	}
	kv := getResp.Kvs[0]
	return decode(s.codec, s.versioner, kv.Value, out, kv.ModRevision)
}

// Create implements storage.Interface.Create.
func (s *store) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	if version, err := s.versioner.ObjectResourceVersion(obj); err == nil && version != 0 {
		return errors.New("resourceVersion should not be set on objects to be created")
	}
	data, err := runtime.Encode(s.codec, obj)
	if err != nil {
		return err
	}
	key = keyWithPrefix(s.pathPrefix, key)

	opts, err := s.ttlOpts(ctx, int64(ttl))
	if err != nil {
		return err
	}

	txnResp, err := s.client.KV.Txn(ctx).If(
		notFound(key),
	).Then(
		clientv3.OpPut(key, string(data), opts...),
	).Commit()
	if err != nil {
		return err
	}
	if !txnResp.Succeeded {
		return storage.NewKeyExistsError(key, 0)
	}

	if out != nil {
		putResp := txnResp.Responses[0].GetResponsePut()
		return decode(s.codec, s.versioner, data, out, putResp.Header.Revision)
	}
	return nil
}

// Delete implements storage.Interface.Delete.
func (s *store) Delete(ctx context.Context, key string, out runtime.Object, precondtions *storage.Preconditions) error {
	v, err := conversion.EnforcePtr(out)
	if err != nil {
		panic("unable to convert output object to pointer")
	}
	key = keyWithPrefix(s.pathPrefix, key)
	if precondtions == nil {
		return s.unconditionalDelete(ctx, key, out)
	}
	return s.conditionalDelete(ctx, key, out, v, precondtions)
}

func (s *store) unconditionalDelete(ctx context.Context, key string, out runtime.Object) error {
	// We need to do get and delete in single transaction in order to
	// know the value and revision before deleting it.
	txnResp, err := s.client.KV.Txn(ctx).If().Then(
		clientv3.OpGet(key),
		clientv3.OpDelete(key),
	).Commit()
	if err != nil {
		return err
	}
	getResp := txnResp.Responses[0].GetResponseRange()
	if len(getResp.Kvs) == 0 {
		return storage.NewKeyNotFoundError(key, 0)
	}

	kv := getResp.Kvs[0]
	return decode(s.codec, s.versioner, kv.Value, out, kv.ModRevision)
}

func (s *store) conditionalDelete(ctx context.Context, key string, out runtime.Object, v reflect.Value, precondtions *storage.Preconditions) error {
	getResp, err := s.client.KV.Get(ctx, key)
	if err != nil {
		return err
	}
	for {
		origState, err := s.getState(getResp, key, v, false)
		if err != nil {
			return err
		}
		if err := checkPreconditions(key, precondtions, origState.obj); err != nil {
			return err
		}
		txnResp, err := s.client.KV.Txn(ctx).If(
			clientv3.Compare(clientv3.ModRevision(key), "=", origState.rev),
		).Then(
			clientv3.OpDelete(key),
		).Else(
			clientv3.OpGet(key),
		).Commit()
		if err != nil {
			return err
		}
		if !txnResp.Succeeded {
			getResp = (*clientv3.GetResponse)(txnResp.Responses[0].GetResponseRange())
			glog.V(4).Infof("deletion of %s failed because of a conflict, going to retry", key)
			continue
		}
		return decode(s.codec, s.versioner, origState.data, out, origState.rev)
	}
}

// GuaranteedUpdate implements storage.Interface.GuaranteedUpdate.
func (s *store) GuaranteedUpdate(
	ctx context.Context, key string, out runtime.Object, ignoreNotFound bool,
	precondtions *storage.Preconditions, tryUpdate storage.UpdateFunc, suggestion ...runtime.Object) error {
	trace := util.NewTrace(fmt.Sprintf("GuaranteedUpdate etcd3: %s", reflect.TypeOf(out).String()))
	defer trace.LogIfLong(500 * time.Millisecond)

	v, err := conversion.EnforcePtr(out)
	if err != nil {
		panic("unable to convert output object to pointer")
	}
	key = keyWithPrefix(s.pathPrefix, key)

	var origState *objState
	if len(suggestion) == 1 && suggestion[0] != nil {
		origState, err = s.getStateFromObject(suggestion[0])
		if err != nil {
			return err
		}
	} else {
		getResp, err := s.client.KV.Get(ctx, key, s.getOps...)
		if err != nil {
			return err
		}
		origState, err = s.getState(getResp, key, v, ignoreNotFound)
		if err != nil {
			return err
		}
	}
	trace.Step("initial value restored")

	for {
		if err := checkPreconditions(key, precondtions, origState.obj); err != nil {
			return err
		}

		ret, ttl, err := s.updateState(origState, tryUpdate)
		if err != nil {
			return err
		}

		data, err := runtime.Encode(s.codec, ret)
		if err != nil {
			return err
		}
		if bytes.Equal(data, origState.data) {
			return decode(s.codec, s.versioner, origState.data, out, origState.rev)
		}

		opts, err := s.ttlOpts(ctx, int64(ttl))
		if err != nil {
			return err
		}
		trace.Step("Transaction prepared")

		txnResp, err := s.client.KV.Txn(ctx).If(
			clientv3.Compare(clientv3.ModRevision(key), "=", origState.rev),
		).Then(
			clientv3.OpPut(key, string(data), opts...),
		).Else(
			clientv3.OpGet(key),
		).Commit()
		if err != nil {
			return err
		}
		trace.Step("Transaction committed")
		if !txnResp.Succeeded {
			getResp := (*clientv3.GetResponse)(txnResp.Responses[0].GetResponseRange())
			glog.V(4).Infof("GuaranteedUpdate of %s failed because of a conflict, going to retry", key)
			origState, err = s.getState(getResp, key, v, ignoreNotFound)
			if err != nil {
				return err
			}
			trace.Step("Retry value restored")
			continue
		}
		putResp := txnResp.Responses[0].GetResponsePut()
		return decode(s.codec, s.versioner, data, out, putResp.Header.Revision)
	}
}

// GetToList implements storage.Interface.GetToList.
func (s *store) GetToList(ctx context.Context, key string, resourceVersion string, pred storage.SelectionPredicate, listObj runtime.Object) error {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	key = keyWithPrefix(s.pathPrefix, key)

	getResp, err := s.client.KV.Get(ctx, key, s.getOps...)
	if err != nil {
		return err
	}
	if len(getResp.Kvs) == 0 {
		return nil
	}
	elems := []*elemForDecode{{
		data: getResp.Kvs[0].Value,
		rev:  uint64(getResp.Kvs[0].ModRevision),
	}}
	if err := decodeList(elems, storage.SimpleFilter(pred), listPtr, s.codec, s.versioner); err != nil {
		return err
	}
	// update version with cluster level revision
	return s.versioner.UpdateList(listObj, uint64(getResp.Header.Revision))
}

// List implements storage.Interface.List.
func (s *store) List(ctx context.Context, key, resourceVersion string, pred storage.SelectionPredicate, listObj runtime.Object) error {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	key = keyWithPrefix(s.pathPrefix, key)
	// We need to make sure the key ended with "/" so that we only get children "directories".
	// e.g. if we have key "/a", "/a/b", "/ab", getting keys with prefix "/a" will return all three,
	// while with prefix "/a/" will return only "/a/b" which is the correct answer.
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	getResp, err := s.client.KV.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	elems := make([]*elemForDecode, len(getResp.Kvs))
	for i, kv := range getResp.Kvs {
		elems[i] = &elemForDecode{
			data: kv.Value,
			rev:  uint64(kv.ModRevision),
		}
	}
	if err := decodeList(elems, storage.SimpleFilter(pred), listPtr, s.codec, s.versioner); err != nil {
		return err
	}
	// update version with cluster level revision
	return s.versioner.UpdateList(listObj, uint64(getResp.Header.Revision))
}

// Watch implements storage.Interface.Watch.
func (s *store) Watch(ctx context.Context, key string, resourceVersion string, pred storage.SelectionPredicate) (watch.Interface, error) {
	return s.watch(ctx, key, resourceVersion, pred, false)
}

// WatchList implements storage.Interface.WatchList.
func (s *store) WatchList(ctx context.Context, key string, resourceVersion string, pred storage.SelectionPredicate) (watch.Interface, error) {
	return s.watch(ctx, key, resourceVersion, pred, true)
}

func (s *store) watch(ctx context.Context, key string, rv string, pred storage.SelectionPredicate, recursive bool) (watch.Interface, error) {
	rev, err := storage.ParseWatchResourceVersion(rv)
	if err != nil {
		return nil, err
	}
	key = keyWithPrefix(s.pathPrefix, key)
	return s.watcher.Watch(ctx, key, int64(rev), recursive, pred)
}

func (s *store) getState(getResp *clientv3.GetResponse, key string, v reflect.Value, ignoreNotFound bool) (*objState, error) {
	state := &objState{
		obj:  reflect.New(v.Type()).Interface().(runtime.Object),
		meta: &storage.ResponseMeta{},
	}
	if len(getResp.Kvs) == 0 {
		if !ignoreNotFound {
			return nil, storage.NewKeyNotFoundError(key, 0)
		}
		if err := runtime.SetZeroValue(state.obj); err != nil {
			return nil, err
		}
	} else {
		state.rev = getResp.Kvs[0].ModRevision
		state.meta.ResourceVersion = uint64(state.rev)
		state.data = getResp.Kvs[0].Value
		if err := decode(s.codec, s.versioner, state.data, state.obj, state.rev); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func (s *store) getStateFromObject(obj runtime.Object) (*objState, error) {
	state := &objState{
		obj:  obj,
		meta: &storage.ResponseMeta{},
	}

	rv, err := s.versioner.ObjectResourceVersion(obj)
	if err != nil {
		return nil, fmt.Errorf("couldn't get resource version: %v", err)
	}
	state.rev = int64(rv)
	state.meta.ResourceVersion = uint64(state.rev)

	// Compute the serialized form - for that we need to temporarily clean
	// its resource version field (those are not stored in etcd).
	if err := s.versioner.UpdateObject(obj, 0); err != nil {
		return nil, errors.New("resourceVersion cannot be set on objects store in etcd")
	}
	state.data, err = runtime.Encode(s.codec, obj)
	if err != nil {
		return nil, err
	}
	s.versioner.UpdateObject(state.obj, uint64(rv))
	return state, nil
}

func (s *store) updateState(st *objState, userUpdate storage.UpdateFunc) (runtime.Object, uint64, error) {
	ret, ttlPtr, err := userUpdate(st.obj, *st.meta)
	if err != nil {
		return nil, 0, err
	}

	version, err := s.versioner.ObjectResourceVersion(ret)
	if err != nil {
		return nil, 0, err
	}
	if version != 0 {
		// We cannot store object with resourceVersion in etcd. We need to reset it.
		if err := s.versioner.UpdateObject(ret, 0); err != nil {
			return nil, 0, fmt.Errorf("UpdateObject failed: %v", err)
		}
	}
	var ttl uint64
	if ttlPtr != nil {
		ttl = *ttlPtr
	}
	return ret, ttl, nil
}

// ttlOpts returns client options based on given ttl.
// ttl: if ttl is non-zero, it will attach the key to a lease with ttl of roughly the same length
func (s *store) ttlOpts(ctx context.Context, ttl int64) ([]clientv3.OpOption, error) {
	if ttl == 0 {
		return nil, nil
	}
	// TODO: one lease per ttl key is expensive. Based on current use case, we can have a long window to
	// put keys within into same lease. We shall benchmark this and optimize the performance.
	lcr, err := s.client.Lease.Grant(ctx, ttl)
	if err != nil {
		return nil, err
	}
	return []clientv3.OpOption{clientv3.WithLease(clientv3.LeaseID(lcr.ID))}, nil
}

func keyWithPrefix(prefix, key string) string {
	if strings.HasPrefix(key, prefix) {
		return key
	}
	return path.Join(prefix, key)
}

// decode decodes value of bytes into object. It will also set the object resource version to rev.
// On success, objPtr would be set to the object.
func decode(codec runtime.Codec, versioner storage.Versioner, value []byte, objPtr runtime.Object, rev int64) error {
	if _, err := conversion.EnforcePtr(objPtr); err != nil {
		panic("unable to convert output object to pointer")
	}
	_, _, err := codec.Decode(value, nil, objPtr)
	if err != nil {
		return err
	}
	// being unable to set the version does not prevent the object from being extracted
	versioner.UpdateObject(objPtr, uint64(rev))
	return nil
}

// decodeList decodes a list of values into a list of objects, with resource version set to corresponding rev.
// On success, ListPtr would be set to the list of objects.
func decodeList(elems []*elemForDecode, filter storage.FilterFunc, ListPtr interface{}, codec runtime.Codec, versioner storage.Versioner) error {
	v, err := conversion.EnforcePtr(ListPtr)
	if err != nil || v.Kind() != reflect.Slice {
		panic("need ptr to slice")
	}
	for _, elem := range elems {
		obj, _, err := codec.Decode(elem.data, nil, reflect.New(v.Type().Elem()).Interface().(runtime.Object))
		if err != nil {
			return err
		}
		// being unable to set the version does not prevent the object from being extracted
		versioner.UpdateObject(obj, elem.rev)
		if filter(obj) {
			v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
		}
	}
	return nil
}

func checkPreconditions(key string, preconditions *storage.Preconditions, out runtime.Object) error {
	if preconditions == nil {
		return nil
	}
	objMeta, err := api.ObjectMetaFor(out)
	if err != nil {
		return storage.NewInternalErrorf("can't enforce preconditions %v on un-introspectable object %v, got error: %v", *preconditions, out, err)
	}
	if preconditions.UID != nil && *preconditions.UID != objMeta.UID {
		errMsg := fmt.Sprintf("Precondition failed: UID in precondition: %v, UID in object meta: %v", *preconditions.UID, objMeta.UID)
		return storage.NewInvalidObjError(key, errMsg)
	}
	return nil
}

func notFound(key string) clientv3.Cmp {
	return clientv3.Compare(clientv3.ModRevision(key), "=", 0)
}
