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
	"path/filepath"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/util/container"
)

const (
	defaultIndexImage        = "quay.io/operator-framework/upstream-opm-builder:latest"
	defaultContainerName     = "registry-grpc"
	defaultContainerPortName = "grpc"
	defaultGRPCPort          = 50051
	semverBundleAddMode      = "semver"
	replacesBundleAddMode    = "replaces"
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

	// GRPCPort is the container grpc port which is defaulted to 50051
	GRPCPort int32
}

// newRegistryPod initializes the RegistryPod struct and sets defaults for empty fields
func newRegistryPod(kubeclient kubernetes.Interface, dbPath, bundleImage, namespace string) *RegistryPod {
	rp := &RegistryPod{}

	if rp.GRPCPort == 0 {
		rp.GRPCPort = defaultGRPCPort
	}

	if len(rp.IndexImage) == 0 {
		rp.IndexImage = defaultIndexImage
	}

	if len(rp.BundleAddMode) == 0 {
		if rp.IndexImage == defaultIndexImage {
			rp.BundleAddMode = semverBundleAddMode
		} else {
			rp.BundleAddMode = replacesBundleAddMode
		}
	}
	rp.Kubeclient = kubeclient
	rp.DBPath = dbPath
	rp.BundleImage = bundleImage
	rp.Namespace = namespace

	return rp
}

// CreateOnCluster returns a bundle registry pod built from an index image
// after verifying successful creation of the pod, or returns the pod logs and error in case of failures
func (rp *RegistryPod) CreateOnCluster(ctx context.Context) (*corev1.Pod, string, error) {
	// validate the RegistryPod struct and ensure required fields are set
	err := rp.validate()
	if err != nil {
		return nil, "", fmt.Errorf("error in validating RegistryPod struct: %v", err)
	}

	// create the registry pod on the cluster
	pod, err := rp.createPodOnCluster(ctx)
	if err != nil {
		podLogs, logErr := rp.FetchPodLogs(ctx, pod)
		if logErr != nil {
			return nil, podLogs, fmt.Errorf("error creating pod: %v: and fetching logs: %v", err, logErr)
		}
		return nil, podLogs, fmt.Errorf("error creating pod: %v", err)
	}

	// verify that pod is successfully created
	err = rp.validatePodOnCluster(ctx, pod)
	if err != nil {
		podLogs, logErr := rp.FetchPodLogs(ctx, pod)
		if logErr != nil {
			return nil, podLogs, fmt.Errorf("error verifying pod creation: %v: and fetching logs: %v", err, logErr)
		}
		return nil, podLogs, fmt.Errorf("error verifying pod creation: %v", err)
	}

	return pod, "", nil
}

// createPodOnCluster returns a pod built from an index image or returns if an existing pod with the same name is found
func (rp *RegistryPod) createPodOnCluster(ctx context.Context) (*corev1.Pod, error) {
	// call podForBundleRegistry() to make the pod definition
	pod, err := rp.podForBundleRegistry()
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

// validatePodOnCluster polls and verifies that the pod status is running
func (rp *RegistryPod) validatePodOnCluster(ctx context.Context, pod *corev1.Pod) error {
	// Upon creation of new pod, poll and verify that pod status is running
	podCheck := wait.ConditionFunc(func() (done bool, err error) {
		p, err := rp.Kubeclient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error getting pod %s: %w", pod.Name, err)
		}
		return p.Status.Phase == corev1.PodRunning, nil
	})

	// poll every 200 ms until podCheck is true or context is done
	err := wait.PollImmediateUntil(time.Duration(200*time.Millisecond), podCheck, ctx.Done())
	if err != nil {
		return fmt.Errorf("error waiting for registry pod %s to run: %v", pod.Name, err)
	}

	return err
}

// validate will ensure that RegistryPod required fields are set
// and throws error if not set
func (rp *RegistryPod) validate() error {
	if len(rp.BundleImage) == 0 {
		return errors.New("bundle image cannot be empty")
	}
	if len(rp.DBPath) == 0 {
		return errors.New("registry database path cannot be empty")
	}

	if len(rp.Namespace) == 0 {
		return errors.New("pod namespace cannot be empty")
	}

	if len(rp.BundleAddMode) == 0 {
		panic("bundle add mode cannot be empty")
	}
	if rp.BundleAddMode != semverBundleAddMode && rp.BundleAddMode != replacesBundleAddMode {
		return errors.New("invalid bundle mode")
	}

	return nil
}

// getPodName will return a string constructed from the bundle Image name
func getPodName(bundleImage string) (string, error) {
	image, err := container.ImageFromString(bundleImage)
	if err != nil {
		return "", fmt.Errorf("invalid bundle image name: %s", bundleImage)
	}

	return image.Name + "-index", nil
}

// podForBundleRegistry constructs and returns the registry pod definition
func (rp *RegistryPod) podForBundleRegistry() (*corev1.Pod, error) {
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
						{Name: defaultContainerPortName, ContainerPort: rp.GRPCPort},
					},
				},
			},
		},
	}

	return pod, nil
}

// getContainerCmd uses templating to construct the container command
func (rp *RegistryPod) getContainerCmd() (string, error) {
	const containerCommand = "/bin/mkdir -p {{ .DBPath | basename }} &&" +
		"/bin/opm registry add -d {{ .DBPath | basename }} -b {{.BundleImage}} --mode={{.BundleAddMode}} &&" +
		"/bin/opm registry serve -d {{ .DBPath | basename }} -p {{.GRPCPort}}"
	type bundleCmd struct {
		BundleImage, DBPath, BundleAddMode string
		GRPCPort                           int32
	}

	var command = bundleCmd{rp.BundleImage, rp.DBPath, rp.BundleAddMode, rp.GRPCPort}

	out := &bytes.Buffer{}

	// create a custom basename template function
	funcMap := template.FuncMap{
		"basename": filepath.Base,
	}

	// add the custom basename template function to the
	// template's FuncMap and parse the containerCommand
	tmp := template.Must(template.New("containerCommand").Funcs(funcMap).Parse(containerCommand))

	err := tmp.Execute(out, command)
	if err != nil {
		return "", fmt.Errorf("error in parsing container command: %w", err)
	}

	return out.String(), nil
}

// FetchPodLogs gets the logs from the registry pod
func (rp *RegistryPod) FetchPodLogs(ctx context.Context, pod *corev1.Pod) (string, error) {
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
