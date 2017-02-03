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

package kubelet

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/clock"
)

// mockPodStatusProvider returns the status on the specified pod
type mockPodStatusProvider struct {
	pods []*api.Pod
}

// GetPodStatus returns the status on the associated pod with matching uid (if found)
func (m *mockPodStatusProvider) GetPodStatus(uid types.UID) (api.PodStatus, bool) {
	for _, pod := range m.pods {
		if pod.UID == uid {
			return pod.Status, true
		}
	}
	return api.PodStatus{}, false
}

// TestActiveDeadlineHandler verifies the active deadline handler functions as expected.
func TestActiveDeadlineHandler(t *testing.T) {
	pods := newTestPods(4)
	fakeClock := clock.NewFakeClock(time.Now())
	podStatusProvider := &mockPodStatusProvider{pods: pods}
	fakeRecorder := &record.FakeRecorder{}
	handler, err := newActiveDeadlineHandler(podStatusProvider, fakeRecorder, fakeClock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := unversioned.Now()
	startTime := unversioned.NewTime(now.Time.Add(-1 * time.Minute))

	// this pod has exceeded its active deadline
	exceededActiveDeadlineSeconds := int64(30)
	pods[0].Status.StartTime = &startTime
	pods[0].Spec.ActiveDeadlineSeconds = &exceededActiveDeadlineSeconds

	// this pod has not exceeded its active deadline
	notYetActiveDeadlineSeconds := int64(120)
	pods[1].Status.StartTime = &startTime
	pods[1].Spec.ActiveDeadlineSeconds = &notYetActiveDeadlineSeconds

	// this pod has no deadline
	pods[2].Status.StartTime = &startTime
	pods[2].Spec.ActiveDeadlineSeconds = nil

	testCases := []struct {
		pod      *api.Pod
		expected bool
	}{{pods[0], true}, {pods[1], false}, {pods[2], false}, {pods[3], false}}

	for i, testCase := range testCases {
		if actual := handler.ShouldSync(testCase.pod); actual != testCase.expected {
			t.Errorf("[%d] ShouldSync expected %#v, got %#v", i, testCase.expected, actual)
		}
		actual := handler.ShouldEvict(testCase.pod)
		if actual.Evict != testCase.expected {
			t.Errorf("[%d] ShouldEvict.Evict expected %#v, got %#v", i, testCase.expected, actual.Evict)
		}
		if testCase.expected {
			if actual.Reason != reason {
				t.Errorf("[%d] ShouldEvict.Reason expected %#v, got %#v", i, message, actual.Reason)
			}
			if actual.Message != message {
				t.Errorf("[%d] ShouldEvict.Message expected %#v, got %#v", i, message, actual.Message)
			}
		}
	}
}
