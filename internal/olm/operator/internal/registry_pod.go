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

package olm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultIndexImage        = "quay.io/operator-framework/upstream-opm-builder:latest"
	defaultGRPCPort          = 50051
	defaultContainerName     = "registry-grpc"
	defaultContainerPortName = "grpc"
)

// RegistryPod holds resources necessary for creation of a registry server
type RegistryPod struct {
	// BundleAddMode specifies the graph update mode that defines how channel graphs are updated
	BundleAddMode string

	// BundleImage specifies the container image that opm uses to generate and incrementally update the database
	BundleImage string

	// Index image contains a database of pointers to operator manifest content that is queriable via an API.
	// new version of an operator bundle when published can be added to an index image
	IndexImage string

	// DBPath refers to the registry DB;
	// if an index image is provided, the existing registry DB is located at /database/index.db
	DBPath string

	// Namespace refers to the specific namespace in which the registry pod will be created and scoped to
	Namespace string

	// Kubeclient refers to a Kubernetes clientset that implements kubernetes.Interface.
	Kubeclient kubernetes.Interface

	// grpcPort is the container grpc port which is defaulted to 50051
	grpcPort int32
}

// Create returns a bundle registry pod built from an index image after verifying successful creation
func (rp *RegistryPod) Create(ctx context.Context) (*corev1.Pod, error) {
	pod, err := rp.create(ctx)
	if err != nil {
		return nil, fmt.Errorf("error verifying pod creation: %v", err)
	}

	err = rp.verifyPodCreation(ctx, pod)
	if err != nil {
		return nil, fmt.Errorf("error verifying pod creation: %v", err)
	}

	return pod, nil
}

// create returns a pod built from an index image or returns if an existing pod with the same name is found
func (rp *RegistryPod) create(ctx context.Context) (*corev1.Pod, error) {
	// call init() to set defaults and validate the RegistryPod struct
	err := rp.init()
	if err != nil {
		return nil, fmt.Errorf("error in initializing and validating registry pod: %v", err)
	}

	// call build() to make the pod definition
	pod, err := rp.build()
	if err != nil {
		return nil, fmt.Errorf("error in building registry pod: %v", err)
	}

	// Check if Pod already exists
	tmpPod, err := rp.Kubeclient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create pod in kubernetes cluster using the above pod definition and clientset
			pod, err = rp.Kubeclient.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error creating registry pod: %v", err)
			}
		} else {
			return nil, fmt.Errorf("error getting existing registry pod: %v", err)
		}
	} else {
		return tmpPod, nil
	}

	return pod, nil
}

func (rp *RegistryPod) verifyPodCreation(ctx context.Context, pod *corev1.Pod) error {
	// Upon creation of new pod, poll and verify that pod status is running
	podCheck := wait.ConditionFunc(func() (done bool, err error) {
		p, err := rp.Kubeclient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return true, fmt.Errorf("error getting pod %s: %w", pod.Name, err)
		}
		return p.Status.Phase == corev1.PodRunning, nil
	})

	err := wait.PollImmediateUntil(time.Duration(1*time.Second), podCheck, ctx.Done())
	if err != nil {
		return fmt.Errorf("error waiting for registry pod %s to run: %v", pod.Name, err)
	}

	_, err = rp.fetchPodLogs(ctx, pod)
	if err != nil {
		return fmt.Errorf("error in fetching pod logs: %v", err)
	}

	return err
}

func (rp *RegistryPod) init() error {
	rp.setDefaults()
	err := rp.validate()
	if err != nil {
		return fmt.Errorf("error in validating registry pod: %v", err)
	}
	return nil
}

func (rp *RegistryPod) setDefaults() {
	// set the grpcPort to default 50051
	rp.grpcPort = defaultGRPCPort

	if len(rp.IndexImage) == 0 {
		rp.IndexImage = defaultIndexImage
	}

	if len(rp.BundleAddMode) == 0 {
		if rp.IndexImage == defaultIndexImage {
			rp.BundleAddMode = "semver"
		} else {
			rp.BundleAddMode = "replaces"
		}
	}
}

func (rp *RegistryPod) validate() error {
	if len(rp.BundleImage) == 0 {
		return errors.New("bundle image is required")
	}
	if len(rp.DBPath) == 0 {
		return errors.New("registry database path is required")
	}

	if len(rp.Namespace) == 0 {
		return errors.New("pod namespace is required")
	}

	return nil
}

// getPodName will return a string constructed from the bundle Image name
func getPodName(bundleName string) (string, error) {
	var podName string
	if strings.Contains(bundleName, "/") {
		split := strings.Split(bundleName, "/")
		lastSegment := strings.Split(split[len(split)-1:][0], ":")
		// get the last but one element from last segment excluding
		// the version number and append index
		podName = lastSegment[len(lastSegment)-2] + "-index"
	} else {
		return "", fmt.Errorf("invalid bundle image name: %s", bundleName)
	}
	return podName, nil
}

// build sets the defaults and validates the RegistryPod struct, and returns a pod definition
func (rp *RegistryPod) build() (*corev1.Pod, error) {
	podName, err := getPodName(rp.BundleImage)
	if err != nil {
		return nil, fmt.Errorf("error in building pod: %v", err)
	}

	containerCmd, err := rp.getContainerCmd()
	if err != nil {
		return nil, fmt.Errorf("error in parsing container command: %v", err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: rp.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  defaultContainerName,
					Image: rp.IndexImage,
					Command: []string{
						"/bin/sh",
						"-c",
						containerCmd,
					},
					Ports: []corev1.ContainerPort{
						{Name: defaultContainerPortName, ContainerPort: rp.grpcPort},
					},
				},
			},
		},
	}

	return pod, nil
}

func (rp *RegistryPod) getContainerCmd() (string, error) {
	const containerCommand = `
/bin/mkdir -p /database && 
/bin/opm registry add -d {{.DBPath}} -b {{.BundleImage}} --mode={{.BundleAddMode}} &&
/bin/opm registry serve -d {{.DBPath}}
`
	type bundleCmd struct {
		BundleImage, DBPath, BundleAddMode string
	}

	var cmds = []bundleCmd{
		{rp.BundleImage, rp.DBPath, rp.BundleAddMode},
	}

	out := &bytes.Buffer{}

	// Create a new template and parse the containerCommand into it
	tmpl := template.Must(template.New("containerCommand").Parse(containerCommand))

	// Execute the template
	for _, cmd := range cmds {
		err := tmpl.Execute(out, cmd)
		if err != nil {
			return "", fmt.Errorf("error in parsing container command: %w", err)
		}
	}

	return out.String(), nil
}

// fetchPodLogs gets the logs from the registry pod
func (rp *RegistryPod) fetchPodLogs(ctx context.Context, pod *corev1.Pod) (string, error) {
	req := rp.Kubeclient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %v", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %v", err)
	}
	return buf.String(), nil
}
