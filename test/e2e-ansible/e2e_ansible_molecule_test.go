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
// limitations under the License.

package e2e_ansible_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	testutils "github.com/operator-framework/operator-sdk/test/internal"
)

var _ = Describe("Testing ansible projects", func() {
	Context("with molecule", func() {
		//var imageTest = "quay.io/example/ansible-test-operator:v0.0.1"

		It("should run molecule tests", func() {
			By("allowing the project be multigroup")
			err := tc.AllowProjectBeMultiGroup()
			Expect(err).Should(BeNil())

			By("creating secret API")
			err = tc.CreateAPI(
				// the tool do not allow we crate an API with a group nil
				// which is required here to mock the tests.
				// Also, it is an open issue in upstream as well. More info: https://github.com/kubernetes-sigs/kubebuilder/issues/1404
				// and the tests should be changed when the tool allows we create API's for core types.
				// todo: replace the ignore value when the tool provide a solution for it.
				"--group", "ignore",
				"--version", "v1",
				"--kind", "Secret",
				"--generate-role")
			Expect(err).Should(Succeed())

			By("removing ignore group for the secret from watches as an workaround to work with core types")
			testutils.ReplaceInFile(filepath.Join(tc.Dir, "watches.yaml"),
				"ignore.example.com", "\"\"")

			By("removing molecule test for the Secret since it is a core type")
			cmd := exec.Command("rm", "-rf", filepath.Join(tc.Dir, "molecule", "default", "tasks", "secret_test.yml"))
			_, err = tc.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("adding Secret task to the role")
			testutils.ReplaceInFile(filepath.Join(tc.Dir, "roles", "secret", "tasks", "main.yml"),
				originalTaskSecret, taskForSecret)

			By("adding memcached molecule test")
			err = ioutil.WriteFile(filepath.Join(tc.Dir, "molecule", "default", "tasks", "memcached_test.yml"),
				[]byte(memcachedMoleculeTest), 0644)
			Expect(err).Should(BeNil())

			By("adding ManageStatus == false for role secret")
			testutils.ReplaceInFile(filepath.Join(tc.Dir, "watches.yaml"),
				"role: secret", manageStatusFalseForRoleSecret)

			By("adding molecule target to test all")
			err = addMoleculeTarget(tc)
			Expect(err).Should(BeNil())

			By("removing FIXME asserts from foo_test.yml")
			testutils.ReplaceInFile(filepath.Join(tc.Dir, "molecule", "default", "tasks", "foo_test.yml"),
				fixmeAssert, "")

			By("removing FIXME asserts from memfin_test.yml")
			testutils.ReplaceInFile(filepath.Join(tc.Dir, "molecule", "default", "tasks", "memfin_test.yml"),
				fixmeAssert, "")

			By("testing with molecule")
			cmd = exec.Command("make", "molecule-kind")
			output, err := tc.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("The molecule test --all output is %s\n", output)

			//By("building the project image")
			//err = tc.Make("docker-build", "IMG="+imageTest)
			//Expect(err).Should(Succeed())
			//
			//if isRunningOnKind() {
			//	By("loading the operator base image for molecule into Kind cluster")
			//	err = tc.LoadImageToKindClusterWithName(imageTest)
			//	Expect(err).Should(Succeed())
			//}
			//
			//By("Test Ansible Molecule scenario")
			//cmd = exec.Command("make", "molecule", "IMG="+imageTest)
			//output, err = tc.Run(cmd)
			//Expect(err).NotTo(HaveOccurred())
			//fmt.Printf("The molecule test all kind output is %s\n", output)
		})
	})
})

// addMoleculeTarget as helper for we test it.
func addMoleculeTarget(tc testutils.TestContext) error {
	makefileBytes, err := ioutil.ReadFile(filepath.Join(tc.Dir, "Makefile"))
	if err != nil {
		return err
	}

	makefileBytes = append([]byte(moleculeTarget), makefileBytes...)
	err = ioutil.WriteFile(filepath.Join(tc.Dir, "Makefile"), makefileBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}

const moleculeTarget = `
.PHONY: molecule molecule-kind
molecule:
	KUSTOMIZE_PATH=$(KUSTOMIZE) OPERATOR_PULL_POLICY=Never OPERATOR_IMAGE=$(IMG) TEST_OPERATOR_NAMESPACE=osdk-test molecule test --all

molecule-kind: 
	KUSTOMIZE_PATH=$(KUSTOMIZE) TEST_OPERATOR_NAMESPACE=default molecule test --all
`

const originalTaskSecret = `---
# tasks file for Secret
`

const taskForSecret = `- name: Create test service
  community.kubernetes.k8s:
    definition:
      kind: Service
      api_version: v1
      metadata:
        name: test-service
        namespace: default
      spec:
        ports:
        - protocol: TCP
          port: 8332
          targetPort: 8332
          name: rpc

- name: Check if jmespath is installed
  set_fact:
    instance_tags: '{{app | json_query(query)}}'
  vars:
    query: 'app[*]."memcached"'
`

const memcachedMoleculeTest = `---
- name: Load CR
  set_fact:
    custom_resource: "{{ lookup('template', '/'.join([samples_dir, cr_file])) | from_yaml }}"
  vars:
    cr_file: 'ansible_v1alpha1_memcached.yaml'

- name: Create the ansible.example.com/v1alpha1.Memcached
  k8s:
    state: present
    namespace: '{{ namespace }}'
    definition: '{{ custom_resource }}'
    wait: yes
    wait_timeout: 300
    wait_condition:
      type: Running
      reason: Successful
      status: "True"

- name: Wait 2 minutes for memcached deployment
  debug:
    var: deploy
  until:
  - deploy is defined
  - deploy.status is defined
  - deploy.status.replicas is defined
  - deploy.status.replicas == deploy.status.get("availableReplicas", 0)
  retries: 12
  delay: 10
  vars:
    deploy: '{{ lookup("k8s",
      kind="Deployment",
      api_version="apps/v1",
      namespace=namespace,
      label_selector="app=memcached"
    )}}'

- name: Create ConfigMap that the Operator should delete
  k8s:
    definition:
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: deleteme
        namespace: '{{ namespace }}'
      data:
        delete: me

- name: Verify custom status exists
  assert:
    that: debug_cr.status.get("test") == "hello world"
  vars:
    debug_cr: '{{ lookup("k8s",
      kind=custom_resource.kind,
      api_version=custom_resource.apiVersion,
      namespace=namespace,
      resource_name=custom_resource.metadata.name
    )}}'

# This will verify that the secret role was executed
- name: Verify that test-service was created
  assert:
    that: lookup('k8s', kind='Service', api_version='v1', namespace=namespace, resource_name='test-service')

- name: Verify that project testing-foo was created
  assert:
    that: lookup('k8s', kind='Namespace', api_version='v1', resource_name='testing-foo')
  when: "'project.openshift.io' in lookup('k8s', cluster_info='api_groups')"

- when: molecule_yml.scenario.name == "test-local"
  block:
  - name: Restart the operator by killing the pod
    k8s:
      state: absent
      definition:
        api_version: v1
        kind: Pod
        metadata:
          namespace: '{{ namespace }}'
          name: '{{ pod.metadata.name }}'
    vars:
      pod: '{{ q("k8s", api_version="v1", kind="Pod", namespace=namespace, label_selector="name=memcached-operator").0 }}'

  - name: Wait 2 minutes for operator deployment
    debug:
      var: deploy
    until:
    - deploy is defined
    - deploy.status is defined
    - deploy.status.replicas is defined
    - deploy.status.replicas == deploy.status.get("availableReplicas", 0)
    retries: 12
    delay: 10
    vars:
      deploy: '{{ lookup("k8s",
        kind="Deployment",
        api_version="apps/v1",
        namespace=namespace,
        resource_name="memcached-operator"
      )}}'

  - name: Wait for reconciliation to have a chance at finishing
    pause:
      seconds:  15

  - name: Delete the service that is created.
    k8s:
      kind: Service
      api_version: v1
      namespace: '{{ namespace }}'
      name: test-service
      state: absent

  - name: Verify that test-service was re-created
    debug:
      var: service
    until: service
    retries: 12
    delay: 10
    vars:
      service: '{{ lookup("k8s",
        kind="Service",
        api_version="v1",
        namespace=namespace,
        resource_name="test-service",
      )}}'

- name: Delete the custom resource
  k8s:
    state: absent
    namespace: '{{ namespace }}'
    definition: '{{ custom_resource }}'

- name: Wait for the custom resource to be deleted
  k8s_info:
    api_version: '{{ custom_resource.apiVersion }}'
    kind: '{{ custom_resource.kind }}'
    namespace: '{{ namespace }}'
    name: '{{ custom_resource.metadata.name }}'
  register: cr
  retries: 10
  delay: 6
  until: not cr.resources
  failed_when: cr.resources

- name: Verify the Deployment was deleted (wait 30s)
  assert:
    that: not lookup('k8s', kind='Deployment', api_version='apps/v1', namespace=namespace, label_selector='app=memcached')
  retries: 10
  delay: 3
`

const manageStatusFalseForRoleSecret = `role: secret
  manageStatus: false`

const fixmeAssert = `
- name: Add assertions here
  assert:
    that: false
    fail_msg: FIXME Add real assertions for your operator
`
