//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kubernetes

import (
	"fmt"

	"github.com/heketi/heketi/v10/pkg/glusterfs/api"

	kubeapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// VolumeGidAnnotationKey is the of the annotation on the PersistentVolume
// object that specifies a supplemental GID.
const VolumeGidAnnotationKey = "pv.beta.kubernetes.io/gid"

func VolumeToPv(volume *api.VolumeInfoResponse,
	name, endpoint string) *kubeapi.PersistentVolume {
	// Initialize object
	pv := &kubeapi.PersistentVolume{}
	pv.Kind = "PersistentVolume"
	pv.APIVersion = "v1"
	pv.Spec.PersistentVolumeReclaimPolicy = kubeapi.PersistentVolumeReclaimRetain
	pv.Spec.AccessModes = []kubeapi.PersistentVolumeAccessMode{
		kubeapi.ReadWriteMany,
	}
	pv.Spec.Capacity = make(kubeapi.ResourceList)
	pv.Spec.Glusterfs = &kubeapi.GlusterfsPersistentVolumeSource{}

	// Set path
	pv.Spec.Capacity[kubeapi.ResourceStorage] =
		resource.MustParse(fmt.Sprintf("%vGi", volume.Size))
	pv.Spec.Glusterfs.Path = volume.Name

	// Set name
	if name == "" {
		pv.ObjectMeta.Name = "glusterfs-" + volume.Id[:8]
	} else {
		pv.ObjectMeta.Name = name

	}

	// Set endpoint
	if endpoint == "" {
		pv.Spec.Glusterfs.EndpointsName = "TYPE ENDPOINT HERE"
	} else {
		pv.Spec.Glusterfs.EndpointsName = endpoint
	}

	gidStr := fmt.Sprintf("%v", volume.Gid)
	pv.Annotations = map[string]string{
		VolumeGidAnnotationKey: gidStr,
	}

	return pv
}
