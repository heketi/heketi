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

package certificates

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	unversionedcore "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/runtime"
	certutil "k8s.io/kubernetes/pkg/util/cert"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/golang/glog"
)

type CertificateController struct {
	kubeClient clientset.Interface

	// CSR framework and store
	csrController *cache.Controller
	csrStore      cache.StoreToCertificateRequestLister

	syncHandler func(csrKey string) error

	approveAllKubeletCSRsForGroup string

	signer *local.Signer

	queue workqueue.RateLimitingInterface
}

func NewCertificateController(kubeClient clientset.Interface, syncPeriod time.Duration, caCertFile, caKeyFile string, approveAllKubeletCSRsForGroup string) (*CertificateController, error) {
	// Send events to the apiserver
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&unversionedcore.EventSinkImpl{Interface: kubeClient.Core().Events("")})

	// Configure cfssl signer
	// TODO: support non-default policy and remote/pkcs11 signing
	policy := &config.Signing{
		Default: config.DefaultConfig(),
	}
	ca, err := local.NewSignerFromFile(caCertFile, caKeyFile, policy)
	if err != nil {
		return nil, err
	}

	cc := &CertificateController{
		kubeClient: kubeClient,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "certificate"),
		signer:     ca,
		approveAllKubeletCSRsForGroup: approveAllKubeletCSRsForGroup,
	}

	// Manage the addition/update of certificate requests
	cc.csrStore.Store, cc.csrController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return cc.kubeClient.Certificates().CertificateSigningRequests().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return cc.kubeClient.Certificates().CertificateSigningRequests().Watch(options)
			},
		},
		&certificates.CertificateSigningRequest{},
		syncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				csr := obj.(*certificates.CertificateSigningRequest)
				glog.V(4).Infof("Adding certificate request %s", csr.Name)
				cc.enqueueCertificateRequest(obj)
			},
			UpdateFunc: func(old, new interface{}) {
				oldCSR := old.(*certificates.CertificateSigningRequest)
				glog.V(4).Infof("Updating certificate request %s", oldCSR.Name)
				cc.enqueueCertificateRequest(new)
			},
			DeleteFunc: func(obj interface{}) {
				csr, ok := obj.(*certificates.CertificateSigningRequest)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						glog.V(2).Infof("Couldn't get object from tombstone %#v", obj)
						return
					}
					csr, ok = tombstone.Obj.(*certificates.CertificateSigningRequest)
					if !ok {
						glog.V(2).Infof("Tombstone contained object that is not a CSR: %#v", obj)
						return
					}
				}
				glog.V(4).Infof("Deleting certificate request %s", csr.Name)
				cc.enqueueCertificateRequest(obj)
			},
		},
	)
	cc.syncHandler = cc.maybeSignCertificate
	return cc, nil
}

// Run the main goroutine responsible for watching and syncing jobs.
func (cc *CertificateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer cc.queue.ShutDown()

	go cc.csrController.Run(stopCh)

	glog.Infof("Starting certificate controller manager")
	for i := 0; i < workers; i++ {
		go wait.Until(cc.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down certificate controller")
}

// worker runs a thread that dequeues CSRs, handles them, and marks them done.
func (cc *CertificateController) worker() {
	for cc.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (cc *CertificateController) processNextWorkItem() bool {
	cKey, quit := cc.queue.Get()
	if quit {
		return false
	}
	defer cc.queue.Done(cKey)

	err := cc.syncHandler(cKey.(string))
	if err == nil {
		cc.queue.Forget(cKey)
		return true
	}

	cc.queue.AddRateLimited(cKey)
	utilruntime.HandleError(fmt.Errorf("Sync %v failed with : %v", cKey, err))
	return true
}

func (cc *CertificateController) enqueueCertificateRequest(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
		return
	}
	cc.queue.Add(key)
}

// maybeSignCertificate will inspect the certificate request and, if it has
// been approved and meets policy expectations, generate an X509 cert using the
// cluster CA assets. If successful it will update the CSR approve subresource
// with the signed certificate.
func (cc *CertificateController) maybeSignCertificate(key string) error {
	startTime := time.Now()
	defer func() {
		glog.V(4).Infof("Finished syncing certificate request %q (%v)", key, time.Now().Sub(startTime))
	}()
	obj, exists, err := cc.csrStore.Store.GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		glog.V(3).Infof("csr has been deleted: %v", key)
		return nil
	}
	csr := obj.(*certificates.CertificateSigningRequest)

	csr, err = cc.maybeAutoApproveCSR(csr)
	if err != nil {
		return fmt.Errorf("error auto approving csr: %v", err)
	}

	// At this point, the controller needs to:
	// 1. Check the approval conditions
	// 2. Generate a signed certificate
	// 3. Update the Status subresource

	if csr.Status.Certificate == nil && IsCertificateRequestApproved(csr) {
		pemBytes := csr.Spec.Request
		req := signer.SignRequest{Request: string(pemBytes)}
		certBytes, err := cc.signer.Sign(req)
		if err != nil {
			return err
		}
		csr.Status.Certificate = certBytes
	}

	_, err = cc.kubeClient.Certificates().CertificateSigningRequests().UpdateStatus(csr)
	return err
}

func (cc *CertificateController) maybeAutoApproveCSR(csr *certificates.CertificateSigningRequest) (*certificates.CertificateSigningRequest, error) {
	// short-circuit if we're not auto-approving
	if cc.approveAllKubeletCSRsForGroup == "" {
		return csr, nil
	}
	// short-circuit if we're already approved or denied
	if approved, denied := getCertApprovalCondition(&csr.Status); approved || denied {
		return csr, nil
	}

	isKubeletBootstrapGroup := false
	for _, g := range csr.Spec.Groups {
		if g == cc.approveAllKubeletCSRsForGroup {
			isKubeletBootstrapGroup = true
			break
		}
	}
	if !isKubeletBootstrapGroup {
		return csr, nil
	}

	x509cr, err := certutil.ParseCSR(csr)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to parse csr %q: %v", csr.Name, err))
		return csr, nil
	}
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return csr, nil
	}
	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return csr, nil
	}
	if len(x509cr.DNSNames)+len(x509cr.EmailAddresses)+len(x509cr.IPAddresses) != 0 {
		return csr, nil
	}

	csr.Status.Conditions = append(csr.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "Auto approving of all kubelet CSRs is enabled on the controller manager",
	})
	return cc.kubeClient.Certificates().CertificateSigningRequests().UpdateApproval(csr)
}
