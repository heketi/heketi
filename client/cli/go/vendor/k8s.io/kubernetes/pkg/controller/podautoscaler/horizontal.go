/*
Copyright 2015 The Kubernetes Authors.

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
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/v1"
	autoscaling "k8s.io/kubernetes/pkg/apis/autoscaling/v1"
	extensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	unversionedautoscaling "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/autoscaling/v1"
	v1core "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	unversionedextensions "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/record"
)

const (
	// Usage shoud exceed the tolerance before we start downscale or upscale the pods.
	// TODO: make it a flag or HPA spec element.
	tolerance = 0.1

	defaultTargetCPUUtilizationPercentage = 80

	HpaCustomMetricsTargetAnnotationName = "alpha/target.custom-metrics.podautoscaler.kubernetes.io"
	HpaCustomMetricsStatusAnnotationName = "alpha/status.custom-metrics.podautoscaler.kubernetes.io"

	scaleUpLimitFactor  = 2
	scaleUpLimitMinimum = 4
)

func calculateScaleUpLimit(currentReplicas int32) int32 {
	return int32(math.Max(scaleUpLimitFactor*float64(currentReplicas), scaleUpLimitMinimum))
}

type HorizontalController struct {
	scaleNamespacer unversionedextensions.ScalesGetter
	hpaNamespacer   unversionedautoscaling.HorizontalPodAutoscalersGetter

	replicaCalc   *ReplicaCalculator
	eventRecorder record.EventRecorder

	// A store of HPA objects, populated by the controller.
	store cache.Store
	// Watches changes to all HPA objects.
	controller cache.Controller
}

var downscaleForbiddenWindow = 5 * time.Minute
var upscaleForbiddenWindow = 3 * time.Minute

func newInformer(controller *HorizontalController, resyncPeriod time.Duration) (cache.Store, cache.Controller) {
	return cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return controller.hpaNamespacer.HorizontalPodAutoscalers(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return controller.hpaNamespacer.HorizontalPodAutoscalers(metav1.NamespaceAll).Watch(options)
			},
		},
		&autoscaling.HorizontalPodAutoscaler{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				hpa := obj.(*autoscaling.HorizontalPodAutoscaler)
				hasCPUPolicy := hpa.Spec.TargetCPUUtilizationPercentage != nil
				_, hasCustomMetricsPolicy := hpa.Annotations[HpaCustomMetricsTargetAnnotationName]
				if !hasCPUPolicy && !hasCustomMetricsPolicy {
					controller.eventRecorder.Event(hpa, v1.EventTypeNormal, "DefaultPolicy", "No scaling policy specified - will use default one. See documentation for details")
				}
				err := controller.reconcileAutoscaler(hpa)
				if err != nil {
					glog.Warningf("Failed to reconcile %s: %v", hpa.Name, err)
				}
			},
			UpdateFunc: func(old, cur interface{}) {
				hpa := cur.(*autoscaling.HorizontalPodAutoscaler)
				err := controller.reconcileAutoscaler(hpa)
				if err != nil {
					glog.Warningf("Failed to reconcile %s: %v", hpa.Name, err)
				}
			},
			// We are not interested in deletions.
		},
	)
}

func NewHorizontalController(evtNamespacer v1core.EventsGetter, scaleNamespacer unversionedextensions.ScalesGetter, hpaNamespacer unversionedautoscaling.HorizontalPodAutoscalersGetter, replicaCalc *ReplicaCalculator, resyncPeriod time.Duration) *HorizontalController {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: evtNamespacer.Events("")})
	recorder := broadcaster.NewRecorder(v1.EventSource{Component: "horizontal-pod-autoscaler"})

	controller := &HorizontalController{
		replicaCalc:     replicaCalc,
		eventRecorder:   recorder,
		scaleNamespacer: scaleNamespacer,
		hpaNamespacer:   hpaNamespacer,
	}
	store, frameworkController := newInformer(controller, resyncPeriod)
	controller.store = store
	controller.controller = frameworkController

	return controller
}

func (a *HorizontalController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.Infof("Starting HPA Controller")
	go a.controller.Run(stopCh)
	<-stopCh
	glog.Infof("Shutting down HPA Controller")
}

// getLastScaleTime returns the hpa's last scale time or the hpa's creation time if the last scale time is nil.
func getLastScaleTime(hpa *autoscaling.HorizontalPodAutoscaler) time.Time {
	lastScaleTime := hpa.Status.LastScaleTime
	if lastScaleTime == nil {
		lastScaleTime = &hpa.CreationTimestamp
	}
	return lastScaleTime.Time
}

func (a *HorizontalController) computeReplicasForCPUUtilization(hpa *autoscaling.HorizontalPodAutoscaler, scale *extensionsv1beta1.Scale) (int32, *int32, time.Time, error) {
	targetUtilization := int32(defaultTargetCPUUtilizationPercentage)
	if hpa.Spec.TargetCPUUtilizationPercentage != nil {
		targetUtilization = *hpa.Spec.TargetCPUUtilizationPercentage
	}
	currentReplicas := scale.Status.Replicas

	if scale.Status.Selector == nil {
		errMsg := "selector is required"
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "SelectorRequired", errMsg)
		return 0, nil, time.Time{}, fmt.Errorf(errMsg)
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: scale.Status.Selector})
	if err != nil {
		errMsg := fmt.Sprintf("couldn't convert selector string to a corresponding selector object: %v", err)
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "InvalidSelector", errMsg)
		return 0, nil, time.Time{}, fmt.Errorf(errMsg)
	}

	desiredReplicas, utilization, timestamp, err := a.replicaCalc.GetResourceReplicas(currentReplicas, targetUtilization, v1.ResourceCPU, hpa.Namespace, selector)
	if err != nil {
		lastScaleTime := getLastScaleTime(hpa)
		if time.Now().After(lastScaleTime.Add(upscaleForbiddenWindow)) {
			a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedGetMetrics", err.Error())
		} else {
			a.eventRecorder.Event(hpa, v1.EventTypeNormal, "MetricsNotAvailableYet", err.Error())
		}

		return 0, nil, time.Time{}, fmt.Errorf("failed to get CPU utilization: %v", err)
	}

	if desiredReplicas != currentReplicas {
		a.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "DesiredReplicasComputed",
			"Computed the desired num of replicas: %d (avgCPUutil: %d, current replicas: %d)",
			desiredReplicas, utilization, scale.Status.Replicas)
	}

	return desiredReplicas, &utilization, timestamp, nil
}

// computeReplicasForCustomMetrics computes the desired number of replicas based on the CustomMetrics passed in cmAnnotation
// as json-serialized extensions.CustomMetricsTargetList.
// Returns number of replicas, metric which required highest number of replicas,
// status string (also json-serialized extensions.CustomMetricsCurrentStatusList),
// last timestamp of the metrics involved in computations or error, if occurred.
func (a *HorizontalController) computeReplicasForCustomMetrics(hpa *autoscaling.HorizontalPodAutoscaler, scale *extensionsv1beta1.Scale,
	cmAnnotation string) (replicas int32, metric string, status string, timestamp time.Time, err error) {

	if cmAnnotation == "" {
		return
	}

	currentReplicas := scale.Status.Replicas

	var targetList extensionsv1beta1.CustomMetricTargetList
	if err := json.Unmarshal([]byte(cmAnnotation), &targetList); err != nil {
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedParseCustomMetricsAnnotation", err.Error())
		return 0, "", "", time.Time{}, fmt.Errorf("failed to parse custom metrics annotation: %v", err)
	}
	if len(targetList.Items) == 0 {
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "NoCustomMetricsInAnnotation", err.Error())
		return 0, "", "", time.Time{}, fmt.Errorf("no custom metrics in annotation")
	}

	statusList := extensionsv1beta1.CustomMetricCurrentStatusList{
		Items: make([]extensionsv1beta1.CustomMetricCurrentStatus, 0),
	}

	for _, customMetricTarget := range targetList.Items {
		if scale.Status.Selector == nil {
			errMsg := "selector is required"
			a.eventRecorder.Event(hpa, v1.EventTypeWarning, "SelectorRequired", errMsg)
			return 0, "", "", time.Time{}, fmt.Errorf("selector is required")
		}

		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: scale.Status.Selector})
		if err != nil {
			errMsg := fmt.Sprintf("couldn't convert selector string to a corresponding selector object: %v", err)
			a.eventRecorder.Event(hpa, v1.EventTypeWarning, "InvalidSelector", errMsg)
			return 0, "", "", time.Time{}, fmt.Errorf("couldn't convert selector string to a corresponding selector object: %v", err)
		}
		floatTarget := float64(customMetricTarget.TargetValue.MilliValue()) / 1000.0
		replicaCountProposal, utilizationProposal, timestampProposal, err := a.replicaCalc.GetMetricReplicas(currentReplicas, floatTarget, fmt.Sprintf("custom/%s", customMetricTarget.Name), hpa.Namespace, selector)
		if err != nil {
			lastScaleTime := getLastScaleTime(hpa)
			if time.Now().After(lastScaleTime.Add(upscaleForbiddenWindow)) {
				a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedGetCustomMetrics", err.Error())
			} else {
				a.eventRecorder.Event(hpa, v1.EventTypeNormal, "CustomMetricsNotAvailableYet", err.Error())
			}

			return 0, "", "", time.Time{}, fmt.Errorf("failed to get custom metric value: %v", err)
		}

		if replicaCountProposal > replicas {
			timestamp = timestampProposal
			replicas = replicaCountProposal
			metric = fmt.Sprintf("Custom metric %s", customMetricTarget.Name)
		}
		quantity, err := resource.ParseQuantity(fmt.Sprintf("%.3f", utilizationProposal))
		if err != nil {
			a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedSetCustomMetrics", err.Error())
			return 0, "", "", time.Time{}, fmt.Errorf("failed to set custom metric value: %v", err)
		}
		statusList.Items = append(statusList.Items, extensionsv1beta1.CustomMetricCurrentStatus{
			Name:         customMetricTarget.Name,
			CurrentValue: quantity,
		})
	}
	byteStatusList, err := json.Marshal(statusList)
	if err != nil {
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedSerializeCustomMetrics", err.Error())
		return 0, "", "", time.Time{}, fmt.Errorf("failed to serialize custom metric status: %v", err)
	}

	if replicas != currentReplicas {
		a.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "DesiredReplicasComputedCustomMetric",
			"Computed the desired num of replicas: %d, metric: %s, current replicas: %d",
			func() *int32 { i := int32(replicas); return &i }(), metric, scale.Status.Replicas)
	}

	return replicas, metric, string(byteStatusList), timestamp, nil
}

func (a *HorizontalController) reconcileAutoscaler(hpa *autoscaling.HorizontalPodAutoscaler) error {
	reference := fmt.Sprintf("%s/%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)

	scale, err := a.scaleNamespacer.Scales(hpa.Namespace).Get(hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)
	if err != nil {
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedGetScale", err.Error())
		return fmt.Errorf("failed to query scale subresource for %s: %v", reference, err)
	}
	currentReplicas := scale.Status.Replicas

	cpuDesiredReplicas := int32(0)
	cpuCurrentUtilization := new(int32)
	cpuTimestamp := time.Time{}

	cmDesiredReplicas := int32(0)
	cmMetric := ""
	cmStatus := ""
	cmTimestamp := time.Time{}

	desiredReplicas := int32(0)
	rescaleReason := ""
	timestamp := time.Now()

	rescale := true

	if scale.Spec.Replicas == 0 {
		// Autoscaling is disabled for this resource
		desiredReplicas = 0
		rescale = false
	} else if currentReplicas > hpa.Spec.MaxReplicas {
		rescaleReason = "Current number of replicas above Spec.MaxReplicas"
		desiredReplicas = hpa.Spec.MaxReplicas
	} else if hpa.Spec.MinReplicas != nil && currentReplicas < *hpa.Spec.MinReplicas {
		rescaleReason = "Current number of replicas below Spec.MinReplicas"
		desiredReplicas = *hpa.Spec.MinReplicas
	} else if currentReplicas == 0 {
		rescaleReason = "Current number of replicas must be greater than 0"
		desiredReplicas = 1
	} else {
		// All basic scenarios covered, the state should be sane, lets use metrics.
		cmAnnotation, cmAnnotationFound := hpa.Annotations[HpaCustomMetricsTargetAnnotationName]

		if hpa.Spec.TargetCPUUtilizationPercentage != nil || !cmAnnotationFound {
			cpuDesiredReplicas, cpuCurrentUtilization, cpuTimestamp, err = a.computeReplicasForCPUUtilization(hpa, scale)
			if err != nil {
				a.updateCurrentReplicasInStatus(hpa, currentReplicas)
				return fmt.Errorf("failed to compute desired number of replicas based on CPU utilization for %s: %v", reference, err)
			}
		}

		if cmAnnotationFound {
			cmDesiredReplicas, cmMetric, cmStatus, cmTimestamp, err = a.computeReplicasForCustomMetrics(hpa, scale, cmAnnotation)
			if err != nil {
				a.updateCurrentReplicasInStatus(hpa, currentReplicas)
				return fmt.Errorf("failed to compute desired number of replicas based on Custom Metrics for %s: %v", reference, err)
			}
		}

		rescaleMetric := ""
		if cpuDesiredReplicas > desiredReplicas {
			desiredReplicas = cpuDesiredReplicas
			timestamp = cpuTimestamp
			rescaleMetric = "CPU utilization"
		}
		if cmDesiredReplicas > desiredReplicas {
			desiredReplicas = cmDesiredReplicas
			timestamp = cmTimestamp
			rescaleMetric = cmMetric
		}
		if desiredReplicas > currentReplicas {
			rescaleReason = fmt.Sprintf("%s above target", rescaleMetric)
		}
		if desiredReplicas < currentReplicas {
			rescaleReason = "All metrics below target"
		}

		if hpa.Spec.MinReplicas != nil && desiredReplicas < *hpa.Spec.MinReplicas {
			desiredReplicas = *hpa.Spec.MinReplicas
		}

		//  never scale down to 0, reserved for disabling autoscaling
		if desiredReplicas == 0 {
			desiredReplicas = 1
		}

		if desiredReplicas > hpa.Spec.MaxReplicas {
			desiredReplicas = hpa.Spec.MaxReplicas
		}

		// Do not upscale too much to prevent incorrect rapid increase of the number of master replicas caused by
		// bogus CPU usage report from heapster/kubelet (like in issue #32304).
		if desiredReplicas > calculateScaleUpLimit(currentReplicas) {
			desiredReplicas = calculateScaleUpLimit(currentReplicas)
		}

		rescale = shouldScale(hpa, currentReplicas, desiredReplicas, timestamp)
	}

	if rescale {
		scale.Spec.Replicas = desiredReplicas
		_, err = a.scaleNamespacer.Scales(hpa.Namespace).Update(hpa.Spec.ScaleTargetRef.Kind, scale)
		if err != nil {
			a.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedRescale", "New size: %d; reason: %s; error: %v", desiredReplicas, rescaleReason, err.Error())
			return fmt.Errorf("failed to rescale %s: %v", reference, err)
		}
		a.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "SuccessfulRescale", "New size: %d; reason: %s", desiredReplicas, rescaleReason)
		glog.Infof("Successfull rescale of %s, old size: %d, new size: %d, reason: %s",
			hpa.Name, currentReplicas, desiredReplicas, rescaleReason)
	} else {
		desiredReplicas = currentReplicas
	}

	return a.updateStatus(hpa, currentReplicas, desiredReplicas, cpuCurrentUtilization, cmStatus, rescale)
}

func shouldScale(hpa *autoscaling.HorizontalPodAutoscaler, currentReplicas, desiredReplicas int32, timestamp time.Time) bool {
	if desiredReplicas == currentReplicas {
		return false
	}

	if hpa.Status.LastScaleTime == nil {
		return true
	}

	// Going down only if the usageRatio dropped significantly below the target
	// and there was no rescaling in the last downscaleForbiddenWindow.
	if desiredReplicas < currentReplicas && hpa.Status.LastScaleTime.Add(downscaleForbiddenWindow).Before(timestamp) {
		return true
	}

	// Going up only if the usage ratio increased significantly above the target
	// and there was no rescaling in the last upscaleForbiddenWindow.
	if desiredReplicas > currentReplicas && hpa.Status.LastScaleTime.Add(upscaleForbiddenWindow).Before(timestamp) {
		return true
	}
	return false
}

func (a *HorizontalController) updateCurrentReplicasInStatus(hpa *autoscaling.HorizontalPodAutoscaler, currentReplicas int32) {
	err := a.updateStatus(hpa, currentReplicas, hpa.Status.DesiredReplicas, hpa.Status.CurrentCPUUtilizationPercentage, hpa.Annotations[HpaCustomMetricsStatusAnnotationName], false)
	if err != nil {
		glog.Errorf("%v", err)
	}
}

func (a *HorizontalController) updateStatus(hpa *autoscaling.HorizontalPodAutoscaler, currentReplicas, desiredReplicas int32, cpuCurrentUtilization *int32, cmStatus string, rescale bool) error {
	hpa.Status = autoscaling.HorizontalPodAutoscalerStatus{
		CurrentReplicas:                 currentReplicas,
		DesiredReplicas:                 desiredReplicas,
		CurrentCPUUtilizationPercentage: cpuCurrentUtilization,
		LastScaleTime:                   hpa.Status.LastScaleTime,
	}
	if cmStatus != "" {
		hpa.Annotations[HpaCustomMetricsStatusAnnotationName] = cmStatus
	}

	if rescale {
		now := metav1.NewTime(time.Now())
		hpa.Status.LastScaleTime = &now
	}

	_, err := a.hpaNamespacer.HorizontalPodAutoscalers(hpa.Namespace).UpdateStatus(hpa)
	if err != nil {
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedUpdateStatus", err.Error())
		return fmt.Errorf("failed to update status for %s: %v", hpa.Name, err)
	}
	glog.V(2).Infof("Successfully updated status for %s", hpa.Name)
	return nil
}
