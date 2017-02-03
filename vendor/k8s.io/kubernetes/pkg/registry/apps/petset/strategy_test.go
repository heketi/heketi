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

package petset

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/apps"
)

func TestStatefulSetStrategy(t *testing.T) {
	ctx := api.NewDefaultContext()
	if !Strategy.NamespaceScoped() {
		t.Errorf("StatefulSet must be namespace scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("StatefulSet should not allow create on update")
	}

	validSelector := map[string]string{"a": "b"}
	validPodTemplate := api.PodTemplate{
		Template: api.PodTemplateSpec{
			ObjectMeta: api.ObjectMeta{
				Labels: validSelector,
			},
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyAlways,
				DNSPolicy:     api.DNSClusterFirst,
				Containers:    []api.Container{{Name: "abc", Image: "image", ImagePullPolicy: "IfNotPresent"}},
			},
		},
	}
	ps := &apps.StatefulSet{
		ObjectMeta: api.ObjectMeta{Name: "abc", Namespace: api.NamespaceDefault},
		Spec: apps.StatefulSetSpec{
			Selector: &unversioned.LabelSelector{MatchLabels: validSelector},
			Template: validPodTemplate.Template,
		},
		Status: apps.StatefulSetStatus{Replicas: 3},
	}

	Strategy.PrepareForCreate(ctx, ps)
	if ps.Status.Replicas != 0 {
		t.Error("StatefulSet should not allow setting status.replicas on create")
	}
	errs := Strategy.Validate(ctx, ps)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	// Just Spec.Replicas is allowed to change
	validPs := &apps.StatefulSet{
		ObjectMeta: api.ObjectMeta{Name: ps.Name, Namespace: ps.Namespace, ResourceVersion: "1", Generation: 1},
		Spec: apps.StatefulSetSpec{
			Selector: ps.Spec.Selector,
			Template: validPodTemplate.Template,
		},
		Status: apps.StatefulSetStatus{Replicas: 4},
	}
	Strategy.PrepareForUpdate(ctx, validPs, ps)
	errs = Strategy.ValidateUpdate(ctx, validPs, ps)
	if len(errs) != 0 {
		t.Errorf("Updating spec.Replicas is allowed on a statefulset: %v", errs)
	}

	validPs.Spec.Selector = &unversioned.LabelSelector{MatchLabels: map[string]string{"a": "bar"}}
	Strategy.PrepareForUpdate(ctx, validPs, ps)
	errs = Strategy.ValidateUpdate(ctx, validPs, ps)
	if len(errs) == 0 {
		t.Errorf("Expected a validation error since updates are disallowed on statefulsets.")
	}
}

func TestStatefulSetStatusStrategy(t *testing.T) {
	ctx := api.NewDefaultContext()
	if !StatusStrategy.NamespaceScoped() {
		t.Errorf("StatefulSet must be namespace scoped")
	}
	if StatusStrategy.AllowCreateOnUpdate() {
		t.Errorf("StatefulSet should not allow create on update")
	}
	validSelector := map[string]string{"a": "b"}
	validPodTemplate := api.PodTemplate{
		Template: api.PodTemplateSpec{
			ObjectMeta: api.ObjectMeta{
				Labels: validSelector,
			},
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyAlways,
				DNSPolicy:     api.DNSClusterFirst,
				Containers:    []api.Container{{Name: "abc", Image: "image", ImagePullPolicy: "IfNotPresent"}},
			},
		},
	}
	oldPS := &apps.StatefulSet{
		ObjectMeta: api.ObjectMeta{Name: "abc", Namespace: api.NamespaceDefault, ResourceVersion: "10"},
		Spec: apps.StatefulSetSpec{
			Replicas: 3,
			Selector: &unversioned.LabelSelector{MatchLabels: validSelector},
			Template: validPodTemplate.Template,
		},
		Status: apps.StatefulSetStatus{
			Replicas: 1,
		},
	}
	newPS := &apps.StatefulSet{
		ObjectMeta: api.ObjectMeta{Name: "abc", Namespace: api.NamespaceDefault, ResourceVersion: "9"},
		Spec: apps.StatefulSetSpec{
			Replicas: 1,
			Selector: &unversioned.LabelSelector{MatchLabels: validSelector},
			Template: validPodTemplate.Template,
		},
		Status: apps.StatefulSetStatus{
			Replicas: 2,
		},
	}
	StatusStrategy.PrepareForUpdate(ctx, newPS, oldPS)
	if newPS.Status.Replicas != 2 {
		t.Errorf("StatefulSet status updates should allow change of pods: %v", newPS.Status.Replicas)
	}
	if newPS.Spec.Replicas != 3 {
		t.Errorf("StatefulSet status updates should not clobber spec: %v", newPS.Spec)
	}
	errs := StatusStrategy.ValidateUpdate(ctx, newPS, oldPS)
	if len(errs) != 0 {
		t.Errorf("Unexpected error %v", errs)
	}
}
