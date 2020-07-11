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
	Describe("creating registry pod", func() {
		Context("with valid values", func() {
			expectedPodName := "example-operator-bundle-index"
			expectedOutput := "/bin/mkdir -p index.db &&" +
				"/bin/opm registry add -d index.db -b quay.io/example/example-operator-bundle:0.2.0 --mode=semver &&" +
				"/bin/opm registry serve -d index.db -p 50051"

			var rp *RegistryPod
			var err error

			BeforeEach(func() {
				rp, err = NewRegistryPod(newFakeClient(), "/database/index.db",
					"quay.io/example/example-operator-bundle:0.2.0", "default")
			})

			It("should create the RegistryPod successfully", func() {
				Expect(err).To(BeNil())
				Expect(rp).NotTo(BeNil())
				Expect(rp.Pod.Name).To(Equal(expectedPodName))
				Expect(rp.Pod.Namespace).To(Equal(rp.Namespace))
				Expect(rp.Pod.Spec.Containers[0].Name).To(Equal(defaultContainerName))
				if len(rp.Pod.Spec.Containers) > 0 {
					if len(rp.Pod.Spec.Containers[0].Ports) > 0 {
						Expect(rp.Pod.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(rp.GRPCPort))
					}
				}
			})

			It("should return a valid container command", func() {
				output, err := rp.getContainerCmd()

				Expect(err).To(BeNil())
				Expect(output).Should(Equal(expectedOutput))
			})

			It("should create the v1corePod on the cluster successfully", func() {
				err := rp.CreateOnCluster()
				Expect(err).To(BeNil())
			})

			It("should have a Pod running on the cluster", func() {
				err := rp.ValidateOnCluster()
				Expect(err).To(BeNil())
				Expect(rp).NotTo(BeNil())
			})
		})

		Context("with an invalid image", func() {
			It("should fail when try to create the RegistryPod", func() {
				expectedErr := "invalid bundle image name"
				_, err := NewRegistryPod(newFakeClient(), "/database/index.db",
					"non/existent/image", "default")

				Expect(err).NotTo(BeNil())
				Expect(err.Error()).Should(ContainSubstring(expectedErr))
			})

			// TODO: add the tests to check the other fails scenarios E.g innvalid bundle option, empty fields and etc
			// TODO: add test to check CreateOnCluster returning error
			// TODO: add test to check ValidateOnCluster returning error
		})
	})
})
