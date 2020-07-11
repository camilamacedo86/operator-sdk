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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// newFakeClient() returns a clientset
func newFakeClient() kubernetes.Interface {
	return fake.NewSimpleClientset()
}

func TestCreateRegistryPod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Registry Pod Suite")
}

var _ = Describe("RegistryPod", func() {
	var rp *RegistryPod

	BeforeEach(func() {
		rp = newRegistryPod(newFakeClient(), "/database/index.db", "quay.io/example/example-operator-bundle:0.2.0", "default")
	})

	Describe("creating registry pod", func() {

		Context("with valid registry pod values", func() {
			var (
				expectedPodName, expectedOutput string
			)

			BeforeEach(func() {
				expectedPodName = "example-operator-bundle-index"
				expectedOutput = "/bin/mkdir -p index.db &&" +
					"/bin/opm registry add -d index.db -b quay.io/example/example-operator-bundle:0.2.0 --mode=semver &&" +
					"/bin/opm registry serve -d index.db -p 50051"
			})

			It("should successfully validate pod", func() {

				Expect(rp.validate()).Should(Succeed())
			})

			It("should construct the pod name from the bundle name successfully", func() {
				podName, err := getPodName(rp.BundleImage)

				Expect(err).To(BeNil())
				Expect(podName).To(Equal(expectedPodName))
			})

			It("should make the pod definition successfully with a valid registry pod", func() {
				pod, err := rp.podForBundleRegistry()

				Expect(err).To(BeNil())
				Expect(pod.Name).To(Equal(expectedPodName))
				Expect(pod.Namespace).To(Equal(rp.Namespace))
				Expect(pod.Spec.Containers[0].Name).To(Equal(defaultContainerName))
				if len(pod.Spec.Containers) > 0 {
					if len(pod.Spec.Containers[0].Ports) > 0 {
						Expect(pod.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(rp.GRPCPort))
					}
				}
			})

			It("should return a valid container command", func() {
				output, err := rp.getContainerCmd()

				Expect(err).To(BeNil())
				Expect(output).Should(Equal(expectedOutput))
			})

			It("should successfully create registry pod without any errors", func() {
				pod, err := rp.createPodOnCluster(context.Background())

				Expect(err).Should(Succeed())
				Expect(pod.Name).To(Equal(expectedPodName))
				Expect(pod.Namespace).To(Equal(rp.Namespace))
				Expect(pod.Spec.Containers[0].Name).To(Equal(defaultContainerName))
				if len(pod.Spec.Containers) > 0 {
					if len(pod.Spec.Containers[0].Ports) > 0 {
						Expect(pod.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(rp.GRPCPort))
					}
				}
			})
		})

		Context("when invalid registry pod values", func() {
			BeforeEach(func() {
				rp.BundleImage = "non/existent/image"
			})

			It("should error upon invalid bundle image", func() {
				expectedErr := "invalid bundle image name"

				_, err := rp.podForBundleRegistry()

				Expect(err).NotTo(BeNil())
				Expect(err.Error()).Should(ContainSubstring(expectedErr))
			})

			It("should not create a registry pod with invalid bundle image", func() {
				exepectedErr := "error in building registry pod"

				_, err := rp.createPodOnCluster(context.Background())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring(exepectedErr))
			})
		})
	})
})
