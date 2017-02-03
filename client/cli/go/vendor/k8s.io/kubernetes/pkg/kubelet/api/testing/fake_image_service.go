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

package testing

import (
	"sync"

	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/util/sliceutils"
)

type FakeImageService struct {
	sync.Mutex

	FakeImageSize uint64
	Called        []string
	Images        map[string]*runtimeApi.Image
}

func (r *FakeImageService) SetFakeImages(images []string) {
	r.Lock()
	defer r.Unlock()

	r.Images = make(map[string]*runtimeApi.Image)
	for _, image := range images {
		r.Images[image] = r.makeFakeImage(image)
	}
}

func (r *FakeImageService) SetFakeImageSize(size uint64) {
	r.Lock()
	defer r.Unlock()

	r.FakeImageSize = size
}

func NewFakeImageService() *FakeImageService {
	return &FakeImageService{
		Called: make([]string, 0),
		Images: make(map[string]*runtimeApi.Image),
	}
}

func (r *FakeImageService) makeFakeImage(image string) *runtimeApi.Image {
	return &runtimeApi.Image{
		Id:       &image,
		Size_:    &r.FakeImageSize,
		RepoTags: []string{image},
	}
}

func (r *FakeImageService) ListImages(filter *runtimeApi.ImageFilter) ([]*runtimeApi.Image, error) {
	r.Lock()
	defer r.Unlock()

	r.Called = append(r.Called, "ListImages")

	images := make([]*runtimeApi.Image, 0)
	for _, img := range r.Images {
		if filter != nil && filter.Image != nil {
			if !sliceutils.StringInSlice(filter.Image.GetImage(), img.RepoTags) {
				continue
			}
		}

		images = append(images, img)
	}
	return images, nil
}

func (r *FakeImageService) ImageStatus(image *runtimeApi.ImageSpec) (*runtimeApi.Image, error) {
	r.Lock()
	defer r.Unlock()

	r.Called = append(r.Called, "ImageStatus")

	return r.Images[image.GetImage()], nil
}

func (r *FakeImageService) PullImage(image *runtimeApi.ImageSpec, auth *runtimeApi.AuthConfig) error {
	r.Lock()
	defer r.Unlock()

	r.Called = append(r.Called, "PullImage")

	// ImageID should be randomized for real container runtime, but here just use
	// image's name for easily making fake images.
	imageID := image.GetImage()
	if _, ok := r.Images[imageID]; !ok {
		r.Images[imageID] = r.makeFakeImage(image.GetImage())
	}

	return nil
}

func (r *FakeImageService) RemoveImage(image *runtimeApi.ImageSpec) error {
	r.Lock()
	defer r.Unlock()

	r.Called = append(r.Called, "RemoveImage")

	// Remove the image
	delete(r.Images, image.GetImage())

	return nil
}
