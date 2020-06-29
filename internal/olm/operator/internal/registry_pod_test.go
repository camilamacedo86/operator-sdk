// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and

package olm

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// newFakeClient() returns a clientset
func newFakeClient() kubernetes.Interface {
	return fake.NewSimpleClientset()
}

func TestCreatePod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test registry pod")
}

var _ = Describe("Creation", func() {
	rp := &RegistryPod{
		Kubeclient:  newFakeClient(),
		DBPath:      "/database/index.db",
		BundleImage: "quay.io/joelanford/example-operator-bundle:0.2.0",
		Namespace:   "default",
	}

	It("validate registry pod", func() {
		err := rp.validate()

		Expect(err).To(BeNil())
	})

	It("create pod", func() {
		pod, err := rp.Kubeclient.CoreV1().Pods("default").Create(context.Background(),
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}}, metav1.CreateOptions{})

		Expect(err).To(BeNil())
		Expect(pod.Name).To(Equal("test-pod"))
	})

	It("get pod name", func() {
		podName, err := getPodName(rp.BundleImage)
		expectedPodName := "example-operator-bundle-index"

		Expect(err).To(BeNil())
		Expect(podName).To(Equal(expectedPodName))
	})

	It("valid bundle image name", func() {
		_, err := rp.build()

		Expect(err).To(BeNil())
	})

	It("invalid bundle image name", func() {
		rp.BundleImage = "invalid-image"
		expectedErr := "invalid bundle image name"

		_, err := rp.build()

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).Should(ContainSubstring(expectedErr))
	})

	It("get container command - valid", func() {
		rp.BundleImage = "quay.io/joelanford/example-operator-bundle:0.2.0"
		output, err := rp.getContainerCmd()

		Expect(err).To(BeNil())
		Expect(output).Should(ContainSubstring(rp.BundleImage))
	})

	It("create registry pod", func() {
		rp.BundleImage = "quay.io/joelanford/example-operator-bundle:0.2.0"
		exepectedPodName := "example-operator-bundle-index"

		pod, err := rp.create(context.Background())
		Expect(err).To(BeNil())
		Expect(pod.Name).To(Equal(exepectedPodName))
	})

	It("create registry pod - invalid", func() {
		rp.BundleImage = "invalid"
		exepectedErr := "error in building registry pod"

		_, err := rp.create(context.Background())
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring(exepectedErr))
	})
})
