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

package ansible

const roleFragment = `
- name: start memcached
  community.kubernetes.k8s:
    definition:
      kind: Deployment
      apiVersion: apps/v1
      metadata:
        name: '{{ ansible_operator_meta.name }}-memcached'
        namespace: '{{ ansible_operator_meta.namespace }}'
      spec:
        replicas: "{{size}}"
        selector:
          matchLabels:
            app: memcached
        template:
          metadata:
            labels:
              app: memcached
          spec:
            containers:
            - name: memcached
              command:
              - memcached
              - -m=64
              - -o
              - modern
              - -v
              image: "docker.io/memcached:1.4.36-alpine"
              ports:
                - containerPort: 11211
`

const defaultsFragment = `size: 1`

const moleculeTaskFragment = `- name: Create the cache.example.com/v1alpha1.Memcached
  k8s:
    state: present
    namespace: "{{ namespace }}"
    definition: "{{ lookup('template', '/'.join([samples_dir, cr_file])) | from_yaml }}"
    wait: yes
    wait_timeout: 300
    wait_condition:
      type: Running
      reason: Successful
      status: "True"
  vars:
    cr_file: 'cache_v1alpha1_memcached.yaml'

- name: Wait 2 minutes for memcached pod to start
  k8s_info:
    kind: "Pod"
    api_version: "v1"
    namespace: "osdk-test"
    label_selectors:
      - app = memcached
  register: pod_list
  until:
    - pod_list.resources is defined
    - pod_list.resources|length == 1
  retries: 12
  delay: 10

- name: Delete memcached pod
  community.kubernetes.k8s:
    state: absent
    definition:
      kind: Pod
      api_version: v1
      metadata:
        namespace: "{{ namespace }}"
        name: "{{ item.metadata.name }}"
  loop: "{{ pod_list.resources }}"

- name: pause
  pause:
    seconds: 10

- name: Wait 2 minutes for memcached pod to restart
  k8s_info:
    kind: "Pod"
    api_version: "v1"
    namespace: "osdk-test"
    label_selectors:
      - app = memcached
  register: pod_list
  until:
    - pod_list.resources is defined
    - pod_list.resources|length == 1
  retries: 12
  delay: 10


- name: Edit Memcached size
  k8s:
    state: present
    namespace: "{{ namespace }}"
    definition:
      apiVersion: cache.example.com/v1alpha1
      kind: Memcached
      metadata:
        name: memcached-sample
      spec:
        size: 3

- name: Wait 2 minutes for 3 memcached pods
  k8s_info:
    kind: "Pod"
    api_version: "v1"
    namespace: "osdk-test"
    label_selectors:
      - app = memcached
  register: pod_list
  until:
    - pod_list.resources is defined
    - pod_list.resources|length == 1
  retries: 12
  delay: 10
`

// false positive: G101: Potential hardcoded credentials (gosec)
// nolint:gosec
const originalTaskSecret = `---
# tasks file for Secret
`

// false positive: G101: Potential hardcoded credentials (gosec)
// nolint:gosec
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

// false positive: G101: Potential hardcoded credentials (gosec)
// nolint:gosec
const manageStatusFalseForRoleSecret = `role: secret
  manageStatus: false`

const fixmeAssert = `
- name: Add assertions here
  assert:
    that: false
    fail_msg: FIXME Add real assertions for your operator
`

const memcachedWithBlackListTask = `
- name: start memcached
  community.kubernetes.k8s:
    definition:
      kind: Deployment
      apiVersion: apps/v1
      metadata:
        name: '{{ ansible_operator_meta.name }}-memcached'
        namespace: '{{ ansible_operator_meta.namespace }}'
        labels:
          app: memcached
      spec:
        replicas: "{{size}}"
        selector:
          matchLabels:
            app: memcached
        template:
          metadata:
            labels:
              app: memcached
          spec:
            containers:
            - name: memcached
              command:
              - memcached
              - -m=64
              - -o
              - modern
              - -v
              image: "docker.io/memcached:1.4.36-alpine"
              ports:
                - containerPort: 11211
              readinessProbe:
                tcpSocket:
                  port: 11211
                initialDelaySeconds: 3
                periodSeconds: 3
- operator_sdk.util.k8s_status:
    api_version: ansible.example.com/v1alpha1
    kind: Memcached
    name: "{{ ansible_operator_meta.name }}"
    namespace: "{{ ansible_operator_meta.namespace }}"
    status:
      test: "hello world"
- community.kubernetes.k8s:
    definition:
      kind: Secret
      apiVersion: v1
      metadata:
        name: test-secret
        namespace: "{{ ansible_operator_meta.namespace }}"
      data:
        test: aGVsbG8K
- name: Get cluster api_groups
  set_fact:
    api_groups: "{{ lookup('community.kubernetes.k8s', cluster_info='api_groups', kubeconfig=lookup('env', 'K8S_AUTH_KUBECONFIG')) }}"
- name: create project if projects are available
  community.kubernetes.k8s:
    definition:
      apiVersion: project.openshift.io/v1
      kind: Project
      metadata:
        name: testing-foo
  when: "'project.openshift.io' in api_groups"
- name: Create ConfigMap to test blacklisted watches
  community.kubernetes.k8s:
    definition:
      kind: ConfigMap
      apiVersion: v1
      metadata:
        name: test-blacklist-watches
        namespace: "{{ ansible_operator_meta.namespace }}"
      data:
        arbitrary: afdasdfsajsafj
    state: present`

const taskToDeleteConfigMap = `- name: delete configmap for test
  community.kubernetes.k8s:
    kind: ConfigMap
    api_version: v1
    name: deleteme
    namespace: default
    state: absent`

const memcachedWatchCustomizations = `playbook: playbooks/memcached.yml
  finalizer:
    name: finalizer.ansible.example.com
    role: memfin
  blacklist:
    - group: ""
      version: v1
      kind: ConfigMap`

const rolesForBaseOperator = `
  ##
  ## Apply customize roles for base operator
  ##
  - apiGroups:
      - ""
    resources:
      - configmaps
      - services
    verbs:
      - create
      - delete
      - get
      - list
      - patch
      - update
      - watch
# +kubebuilder:scaffold:rules
`
