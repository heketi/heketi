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

package downwardapi

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utiltesting "k8s.io/client-go/util/testing"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	"k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/empty_dir"
	volumetest "k8s.io/kubernetes/pkg/volume/testing"
)

const downwardAPIDir = "..data"

func newTestHost(t *testing.T, clientset clientset.Interface) (string, volume.VolumeHost) {
	tempDir, err := utiltesting.MkTmpdir("downwardApi_volume_test.")
	if err != nil {
		t.Fatalf("can't make a temp rootdir: %v", err)
	}
	return tempDir, volumetest.NewFakeVolumeHost(tempDir, clientset, empty_dir.ProbeVolumePlugins())
}

func TestCanSupport(t *testing.T) {
	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, nil)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)

	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	if plugin.GetPluginName() != downwardAPIPluginName {
		t.Errorf("Wrong name: %s", plugin.GetPluginName())
	}
}

func CleanEverything(plugin volume.VolumePlugin, testVolumeName, volumePath string, testPodUID types.UID, t *testing.T) {
	unmounter, err := plugin.NewUnmounter(testVolumeName, testPodUID)
	if err != nil {
		t.Errorf("Failed to make a new Unmounter: %v", err)
	}
	if unmounter == nil {
		t.Errorf("Got a nil Unmounter")
	}

	if err := unmounter.TearDown(); err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if _, err := os.Stat(volumePath); err == nil {
		t.Errorf("TearDown() failed, volume path still exists: %s", volumePath)
	} else if !os.IsNotExist(err) {
		t.Errorf("SetUp() failed: %v", err)
	}
}

func TestLabels(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_labels"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	labels := map[string]string{
		"key1": "value1",
		"key2": "value2"}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels:    labels,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	rootDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(rootDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "labels", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels"}}}},
		},
	}
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Labels: labels}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})

	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()

	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	// downwardAPI volume should create its own empty wrapper path
	podWrapperMetadataDir := fmt.Sprintf("%v/pods/%v/plugins/kubernetes.io~empty-dir/wrapped_%v", rootDir, testPodUID, testVolumeName)

	if _, err := os.Stat(podWrapperMetadataDir); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("SetUp() failed, empty-dir wrapper path was not created: %s", podWrapperMetadataDir)
		} else {
			t.Errorf("SetUp() failed: %v", err)
		}
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "labels"))
	if err != nil {
		t.Errorf(err.Error())
	}
	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(labels)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(labels))
	}

	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)
}

func TestAnnotations(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_annotations"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	annotations := map[string]string{
		"a1": "value1",
		"a2": "value2"}

	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "annotations", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations"}}}},
		},
	}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testName,
			Namespace:   testNamespace,
			Annotations: annotations,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Annotations: annotations}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()

	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "annotations"))
	if err != nil {
		t.Errorf(err.Error())
	}

	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(annotations)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(annotations))
	}
	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)

}

func TestName(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_name"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "name_file_name", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name"}}}},
		},
	}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Name: testName}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()

	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "name_file_name"))
	if err != nil {
		t.Errorf(err.Error())
	}

	if string(data) != testName {
		t.Errorf("Found `%s` expected %s", string(data), testName)
	}

	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)

}

func TestNamespace(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_namespace"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "namespace_file_name", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.namespace"}}}},
		},
	}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Namespace: testNamespace}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()

	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "namespace_file_name"))
	if err != nil {
		t.Errorf(err.Error())
	}
	if string(data) != testNamespace {
		t.Errorf("Found `%s` expected %s", string(data), testNamespace)
	}

	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)

}

func TestWriteTwiceNoUpdate(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_write_twice_no_update"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	labels := map[string]string{
		"key1": "value1",
		"key2": "value2"}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels:    labels,
		},
	})
	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "labels", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels"}}}},
		},
	}
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Labels: labels}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})

	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()
	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	// get the link of the link
	var currentTarget string
	if currentTarget, err = os.Readlink(path.Join(volumePath, downwardAPIDir)); err != nil {
		t.Errorf(".data should be a link... %s\n", err.Error())
	}

	err = mounter.SetUp(nil) // now re-run Setup
	if err != nil {
		t.Errorf("Failed to re-setup volume: %v", err)
	}

	// get the link of the link
	var currentTarget2 string
	if currentTarget2, err = os.Readlink(path.Join(volumePath, downwardAPIDir)); err != nil {
		t.Errorf(".data should be a link... %s\n", err.Error())
	}

	if currentTarget2 != currentTarget {
		t.Errorf("No update between the two Setup... Target link should be the same %s %s\n", currentTarget, currentTarget2)
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "labels"))
	if err != nil {
		t.Errorf(err.Error())
	}

	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(labels)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(labels))
	}
	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)

}

func TestWriteTwiceWithUpdate(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_write_twice_with_update"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	labels := map[string]string{
		"key1": "value1",
		"key2": "value2"}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels:    labels,
		},
	})
	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "labels", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels"}}}},
		},
	}
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Labels: labels}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})

	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()
	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	var currentTarget string
	if currentTarget, err = os.Readlink(path.Join(volumePath, downwardAPIDir)); err != nil {
		t.Errorf("labels file should be a link... %s\n", err.Error())
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "labels"))
	if err != nil {
		t.Errorf(err.Error())
	}

	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(labels)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(labels))
	}

	newLabels := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3"}

	// Now update the labels
	pod.ObjectMeta.Labels = newLabels
	err = mounter.SetUp(nil) // now re-run Setup
	if err != nil {
		t.Errorf("Failed to re-setup volume: %v", err)
	}

	// get the link of the link
	var currentTarget2 string
	if currentTarget2, err = os.Readlink(path.Join(volumePath, downwardAPIDir)); err != nil {
		t.Errorf(".current should be a link... %s\n", err.Error())
	}

	if currentTarget2 == currentTarget {
		t.Errorf("Got and update between the two Setup... Target link should NOT be the same\n")
	}

	data, err = ioutil.ReadFile(path.Join(volumePath, "labels"))
	if err != nil {
		t.Errorf(err.Error())
	}

	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(newLabels)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(newLabels))
	}
	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)
}

func TestWriteWithUnixPath(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_write_with_unix_path"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	labels := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3\n"}

	annotations := map[string]string{
		"a1": "value1",
		"a2": "value2"}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels:    labels,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "this/is/mine/labels", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels"}},
					{Path: "this/is/yours/annotations", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations"}},
				}}},
	}
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Labels: labels, Annotations: annotations}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})

	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()
	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	var data []byte
	data, err = ioutil.ReadFile(path.Join(volumePath, "this/is/mine/labels"))
	if err != nil {
		t.Errorf(err.Error())
	}

	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(labels)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(labels))
	}

	data, err = ioutil.ReadFile(path.Join(volumePath, "this/is/yours/annotations"))
	if err != nil {
		t.Errorf(err.Error())
	}
	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(annotations)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(annotations))
	}
	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)
}

func TestWriteWithUnixPathBadPath(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_write_with_unix_path"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	labels := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels:    labels,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}

	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{
						Path: "this//labels",
						FieldRef: &v1.ObjectFieldSelector{
							FieldPath: "metadata.labels",
						},
					},
				},
			},
		},
	}

	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Labels: labels}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})
	if err != nil {
		t.Fatalf("Failed to make a new Mounter: %v", err)
	} else if mounter == nil {
		t.Fatalf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()
	defer CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)

	err = mounter.SetUp(nil)
	if err != nil {
		t.Fatalf("Failed to setup volume: %v", err)
	}

	data, err := ioutil.ReadFile(path.Join(volumePath, "this/labels"))
	if err != nil {
		t.Fatalf(err.Error())
	}

	if sortLines(string(data)) != sortLines(fieldpath.FormatMap(labels)) {
		t.Errorf("Found `%s` expected %s", data, fieldpath.FormatMap(labels))
	}
}

func TestDefaultMode(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_name"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	defaultMode := int32(0644)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{Path: "name_file_name", FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name"}}}},
		},
	}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Name: testName}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()

	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	fileInfo, err := os.Stat(path.Join(volumePath, "name_file_name"))
	if err != nil {
		t.Errorf(err.Error())
	}

	actualMode := fileInfo.Mode()
	expectedMode := os.FileMode(defaultMode)
	if actualMode != expectedMode {
		t.Errorf("Found mode `%v` expected %v", actualMode, expectedMode)
	}

	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)
}

func TestItemMode(t *testing.T) {
	var (
		testPodUID     = types.UID("test_pod_uid")
		testVolumeName = "test_name"
		testNamespace  = "test_metadata_namespace"
		testName       = "test_metadata_name"
	)

	defaultMode := int32(0644)
	itemMode := int32(0400)
	volumeSpec := &v1.Volume{
		Name: testVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				DefaultMode: &defaultMode,
				Items: []v1.DownwardAPIVolumeFile{
					{
						Path: "name_file_name", FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.name"},
						Mode: &itemMode,
					},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	})

	pluginMgr := volume.VolumePluginMgr{}
	tmpDir, host := newTestHost(t, clientset)
	defer os.RemoveAll(tmpDir)
	pluginMgr.InitPlugins(ProbeVolumePlugins(), host)
	plugin, err := pluginMgr.FindPluginByName(downwardAPIPluginName)
	if err != nil {
		t.Errorf("Can't find the plugin by name")
	}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{UID: testPodUID, Name: testName}}
	mounter, err := plugin.NewMounter(volume.NewSpecFromVolume(volumeSpec), pod, volume.VolumeOptions{})
	if err != nil {
		t.Errorf("Failed to make a new Mounter: %v", err)
	}
	if mounter == nil {
		t.Errorf("Got a nil Mounter")
	}

	volumePath := mounter.GetPath()

	err = mounter.SetUp(nil)
	if err != nil {
		t.Errorf("Failed to setup volume: %v", err)
	}

	fileInfo, err := os.Stat(path.Join(volumePath, "name_file_name"))
	if err != nil {
		t.Errorf(err.Error())
	}

	actualMode := fileInfo.Mode()
	expectedMode := os.FileMode(itemMode)
	if actualMode != expectedMode {
		t.Errorf("Found mode `%v` expected %v", actualMode, expectedMode)
	}

	CleanEverything(plugin, testVolumeName, volumePath, testPodUID, t)
}
