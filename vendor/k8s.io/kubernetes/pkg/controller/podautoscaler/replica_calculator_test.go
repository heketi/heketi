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

package podautoscaler

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	_ "k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	"k8s.io/kubernetes/pkg/runtime"

	heapster "k8s.io/heapster/metrics/api/v1/types"
	metrics_api "k8s.io/heapster/metrics/apis/metrics/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resourceInfo struct {
	name     api.ResourceName
	requests []resource.Quantity
	levels   []int64

	targetUtilization   int32
	expectedUtilization int32
}

type metricInfo struct {
	name   string
	levels []float64

	targetUtilization   float64
	expectedUtilization float64
}

type replicaCalcTestCase struct {
	currentReplicas  int32
	expectedReplicas int32
	expectedError    error

	timestamp time.Time

	resource *resourceInfo
	metric   *metricInfo

	podReadiness []api.ConditionStatus
}

const (
	testNamespace = "test-namespace"
	podNamePrefix = "test-pod"
)

func (tc *replicaCalcTestCase) prepareTestClient(t *testing.T) *fake.Clientset {

	fakeClient := &fake.Clientset{}
	fakeClient.AddReactor("list", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		obj := &api.PodList{}
		for i := 0; i < int(tc.currentReplicas); i++ {
			podReadiness := api.ConditionTrue
			if tc.podReadiness != nil {
				podReadiness = tc.podReadiness[i]
			}
			podName := fmt.Sprintf("%s-%d", podNamePrefix, i)
			pod := api.Pod{
				Status: api.PodStatus{
					Phase: api.PodRunning,
					Conditions: []api.PodCondition{
						{
							Type:   api.PodReady,
							Status: podReadiness,
						},
					},
				},
				ObjectMeta: api.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
					Labels: map[string]string{
						"name": podNamePrefix,
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{{}, {}},
				},
			}

			if tc.resource != nil && i < len(tc.resource.requests) {
				pod.Spec.Containers[0].Resources = api.ResourceRequirements{
					Requests: api.ResourceList{
						tc.resource.name: tc.resource.requests[i],
					},
				}
				pod.Spec.Containers[1].Resources = api.ResourceRequirements{
					Requests: api.ResourceList{
						tc.resource.name: tc.resource.requests[i],
					},
				}
			}
			obj.Items = append(obj.Items, pod)
		}
		return true, obj, nil
	})

	fakeClient.AddProxyReactor("services", func(action core.Action) (handled bool, ret restclient.ResponseWrapper, err error) {
		var heapsterRawMemResponse []byte

		if tc.resource != nil {
			metrics := metrics_api.PodMetricsList{}
			for i, resValue := range tc.resource.levels {
				podMetric := metrics_api.PodMetrics{
					ObjectMeta: v1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%d", podNamePrefix, i),
						Namespace: testNamespace,
					},
					Timestamp: unversioned.Time{Time: tc.timestamp},
					Containers: []metrics_api.ContainerMetrics{
						{
							Name: "container1",
							Usage: v1.ResourceList{
								v1.ResourceName(tc.resource.name): *resource.NewMilliQuantity(
									int64(resValue),
									resource.DecimalSI),
							},
						},
						{
							Name: "container2",
							Usage: v1.ResourceList{
								v1.ResourceName(tc.resource.name): *resource.NewMilliQuantity(
									int64(resValue),
									resource.DecimalSI),
							},
						},
					},
				}
				metrics.Items = append(metrics.Items, podMetric)
			}
			heapsterRawMemResponse, _ = json.Marshal(&metrics)
		} else {
			// only return the pods that we actually asked for
			proxyAction := action.(core.ProxyGetAction)
			pathParts := strings.Split(proxyAction.GetPath(), "/")
			// pathParts should look like [ api, v1, model, namespaces, $NS, pod-list, $PODS, metrics, $METRIC... ]
			if len(pathParts) < 9 {
				return true, nil, fmt.Errorf("invalid heapster path %q", proxyAction.GetPath())
			}

			podNames := strings.Split(pathParts[7], ",")
			podPresent := make([]bool, len(tc.metric.levels))
			for _, name := range podNames {
				if len(name) <= len(podNamePrefix)+1 {
					return true, nil, fmt.Errorf("unknown pod %q", name)
				}
				num, err := strconv.Atoi(name[len(podNamePrefix)+1:])
				if err != nil {
					return true, nil, fmt.Errorf("unknown pod %q", name)
				}
				podPresent[num] = true
			}

			timestamp := tc.timestamp
			metrics := heapster.MetricResultList{}
			for i, level := range tc.metric.levels {
				if !podPresent[i] {
					continue
				}

				metric := heapster.MetricResult{
					Metrics:         []heapster.MetricPoint{{Timestamp: timestamp, Value: uint64(level), FloatValue: &tc.metric.levels[i]}},
					LatestTimestamp: timestamp,
				}
				metrics.Items = append(metrics.Items, metric)
			}
			heapsterRawMemResponse, _ = json.Marshal(&metrics)
		}

		return true, newFakeResponseWrapper(heapsterRawMemResponse), nil
	})

	return fakeClient
}

func (tc *replicaCalcTestCase) runTest(t *testing.T) {
	testClient := tc.prepareTestClient(t)
	metricsClient := metrics.NewHeapsterMetricsClient(testClient, metrics.DefaultHeapsterNamespace, metrics.DefaultHeapsterScheme, metrics.DefaultHeapsterService, metrics.DefaultHeapsterPort)

	replicaCalc := &ReplicaCalculator{
		metricsClient: metricsClient,
		podsGetter:    testClient.Core(),
	}

	selector, err := unversioned.LabelSelectorAsSelector(&unversioned.LabelSelector{
		MatchLabels: map[string]string{"name": podNamePrefix},
	})
	if err != nil {
		require.Nil(t, err, "something went horribly wrong...")
	}

	if tc.resource != nil {
		outReplicas, outUtilization, outTimestamp, err := replicaCalc.GetResourceReplicas(tc.currentReplicas, tc.resource.targetUtilization, tc.resource.name, testNamespace, selector)

		if tc.expectedError != nil {
			require.Error(t, err, "there should be an error calculating the replica count")
			assert.Contains(t, err.Error(), tc.expectedError.Error(), "the error message should have contained the expected error message")
			return
		}
		require.NoError(t, err, "there should not have been an error calculating the replica count")
		assert.Equal(t, tc.expectedReplicas, outReplicas, "replicas should be as expected")
		assert.Equal(t, tc.resource.expectedUtilization, outUtilization, "utilization should be as expected")
		assert.True(t, tc.timestamp.Equal(outTimestamp), "timestamp should be as expected")

	} else {
		outReplicas, outUtilization, outTimestamp, err := replicaCalc.GetMetricReplicas(tc.currentReplicas, tc.metric.targetUtilization, tc.metric.name, testNamespace, selector)

		if tc.expectedError != nil {
			require.Error(t, err, "there should be an error calculating the replica count")
			assert.Contains(t, err.Error(), tc.expectedError.Error(), "the error message should have contained the expected error message")
			return
		}
		require.NoError(t, err, "there should not have been an error calculating the replica count")
		assert.Equal(t, tc.expectedReplicas, outReplicas, "replicas should be as expected")
		assert.InDelta(t, tc.metric.expectedUtilization, 0.1, outUtilization, "utilization should be as expected")
		assert.True(t, tc.timestamp.Equal(outTimestamp), "timestamp should be as expected")
	}
}

func TestReplicaCalcScaleUp(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 5,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{300, 500, 700},

			targetUtilization:   30,
			expectedUtilization: 50,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleUpUnreadyLessScale(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 4,
		podReadiness:     []api.ConditionStatus{api.ConditionFalse, api.ConditionTrue, api.ConditionTrue},
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{300, 500, 700},

			targetUtilization:   30,
			expectedUtilization: 60,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleUpUnreadyNoScale(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 3,
		podReadiness:     []api.ConditionStatus{api.ConditionTrue, api.ConditionFalse, api.ConditionFalse},
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{400, 500, 700},

			targetUtilization:   30,
			expectedUtilization: 40,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleUpCM(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 4,
		metric: &metricInfo{
			name:                "qps",
			levels:              []float64{20.0, 10.0, 30.0},
			targetUtilization:   15.0,
			expectedUtilization: 20.0,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleUpCMUnreadyLessScale(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 4,
		podReadiness:     []api.ConditionStatus{api.ConditionTrue, api.ConditionTrue, api.ConditionFalse},
		metric: &metricInfo{
			name:                "qps",
			levels:              []float64{50.0, 10.0, 30.0},
			targetUtilization:   15.0,
			expectedUtilization: 30.0,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleUpCMUnreadyNoScaleWouldScaleDown(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 3,
		podReadiness:     []api.ConditionStatus{api.ConditionFalse, api.ConditionTrue, api.ConditionFalse},
		metric: &metricInfo{
			name:                "qps",
			levels:              []float64{50.0, 15.0, 30.0},
			targetUtilization:   15.0,
			expectedUtilization: 15.0,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleDown(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  5,
		expectedReplicas: 3,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{100, 300, 500, 250, 250},

			targetUtilization:   50,
			expectedUtilization: 28,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleDownCM(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  5,
		expectedReplicas: 3,
		metric: &metricInfo{
			name:                "qps",
			levels:              []float64{12.0, 12.0, 12.0, 12.0, 12.0},
			targetUtilization:   20.0,
			expectedUtilization: 12.0,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcScaleDownIgnoresUnreadyPods(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  5,
		expectedReplicas: 2,
		podReadiness:     []api.ConditionStatus{api.ConditionTrue, api.ConditionTrue, api.ConditionTrue, api.ConditionFalse, api.ConditionFalse},
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{100, 300, 500, 250, 250},

			targetUtilization:   50,
			expectedUtilization: 30,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcTolerance(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 3,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("0.9"), resource.MustParse("1.0"), resource.MustParse("1.1")},
			levels:   []int64{1010, 1030, 1020},

			targetUtilization:   100,
			expectedUtilization: 102,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcToleranceCM(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 3,
		metric: &metricInfo{
			name:                "qps",
			levels:              []float64{20.0, 21.0, 21.0},
			targetUtilization:   20.0,
			expectedUtilization: 20.66666,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcSuperfluousMetrics(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  4,
		expectedReplicas: 24,
		resource: &resourceInfo{
			name:                api.ResourceCPU,
			requests:            []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:              []int64{4000, 9500, 3000, 7000, 3200, 2000},
			targetUtilization:   100,
			expectedUtilization: 587,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetrics(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  4,
		expectedReplicas: 3,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{400, 95},

			targetUtilization:   100,
			expectedUtilization: 24,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcEmptyMetrics(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas: 4,
		expectedError:   fmt.Errorf("unable to get metrics for resource cpu: no metrics returned from heapster"),
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{},

			targetUtilization: 100,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcEmptyCPURequest(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas: 1,
		expectedError:   fmt.Errorf("missing request for"),
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{},
			levels:   []int64{200},

			targetUtilization: 100,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetricsNoChangeEq(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  2,
		expectedReplicas: 2,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{1000},

			targetUtilization:   100,
			expectedUtilization: 100,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetricsNoChangeGt(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  2,
		expectedReplicas: 2,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{1900},

			targetUtilization:   100,
			expectedUtilization: 190,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetricsNoChangeLt(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  2,
		expectedReplicas: 2,
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{600},

			targetUtilization:   100,
			expectedUtilization: 60,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetricsUnreadyNoChange(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 3,
		podReadiness:     []api.ConditionStatus{api.ConditionFalse, api.ConditionTrue, api.ConditionTrue},
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{100, 450},

			targetUtilization:   50,
			expectedUtilization: 45,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetricsUnreadyScaleUp(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  3,
		expectedReplicas: 4,
		podReadiness:     []api.ConditionStatus{api.ConditionFalse, api.ConditionTrue, api.ConditionTrue},
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{100, 2000},

			targetUtilization:   50,
			expectedUtilization: 200,
		},
	}
	tc.runTest(t)
}

func TestReplicaCalcMissingMetricsUnreadyScaleDown(t *testing.T) {
	tc := replicaCalcTestCase{
		currentReplicas:  4,
		expectedReplicas: 3,
		podReadiness:     []api.ConditionStatus{api.ConditionFalse, api.ConditionTrue, api.ConditionTrue, api.ConditionTrue},
		resource: &resourceInfo{
			name:     api.ResourceCPU,
			requests: []resource.Quantity{resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0"), resource.MustParse("1.0")},
			levels:   []int64{100, 100, 100},

			targetUtilization:   50,
			expectedUtilization: 10,
		},
	}
	tc.runTest(t)
}

// TestComputedToleranceAlgImplementation is a regression test which
// back-calculates a minimal percentage for downscaling based on a small percentage
// increase in pod utilization which is calibrated against the tolerance value.
func TestReplicaCalcComputedToleranceAlgImplementation(t *testing.T) {

	startPods := int32(10)
	// 150 mCPU per pod.
	totalUsedCPUOfAllPods := int64(startPods * 150)
	// Each pod starts out asking for 2X what is really needed.
	// This means we will have a 50% ratio of used/requested
	totalRequestedCPUOfAllPods := int32(2 * totalUsedCPUOfAllPods)
	requestedToUsed := float64(totalRequestedCPUOfAllPods / int32(totalUsedCPUOfAllPods))
	// Spread the amount we ask over 10 pods.  We can add some jitter later in reportedLevels.
	perPodRequested := totalRequestedCPUOfAllPods / startPods

	// Force a minimal scaling event by satisfying  (tolerance < 1 - resourcesUsedRatio).
	target := math.Abs(1/(requestedToUsed*(1-tolerance))) + .01
	finalCpuPercentTarget := int32(target * 100)
	resourcesUsedRatio := float64(totalUsedCPUOfAllPods) / float64(float64(totalRequestedCPUOfAllPods)*target)

	// i.e. .60 * 20 -> scaled down expectation.
	finalPods := int32(math.Ceil(resourcesUsedRatio * float64(startPods)))

	// To breach tolerance we will create a utilization ratio difference of tolerance to usageRatioToleranceValue)
	tc := replicaCalcTestCase{
		currentReplicas:  startPods,
		expectedReplicas: finalPods,
		resource: &resourceInfo{
			name: api.ResourceCPU,
			levels: []int64{
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
				totalUsedCPUOfAllPods / 10,
			},
			requests: []resource.Quantity{
				resource.MustParse(fmt.Sprint(perPodRequested+100) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested-100) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested+10) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested-10) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested+2) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested-2) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested+1) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested-1) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested) + "m"),
				resource.MustParse(fmt.Sprint(perPodRequested) + "m"),
			},

			targetUtilization:   finalCpuPercentTarget,
			expectedUtilization: int32(totalUsedCPUOfAllPods*100) / totalRequestedCPUOfAllPods,
		},
	}

	tc.runTest(t)

	// Reuse the data structure above, now testing "unscaling".
	// Now, we test that no scaling happens if we are in a very close margin to the tolerance
	target = math.Abs(1/(requestedToUsed*(1-tolerance))) + .004
	finalCpuPercentTarget = int32(target * 100)
	tc.resource.targetUtilization = finalCpuPercentTarget
	tc.currentReplicas = startPods
	tc.expectedReplicas = startPods
	tc.runTest(t)
}

// TODO: add more tests
