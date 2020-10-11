#!/usr/bin/env bash

source hack/lib/common.sh
source hack/lib/test_lib.sh
source hack/lib/image_lib.sh

set -o errexit
set -o nounset
set -o pipefail

header_text "Running tests to check ansible molecule"

TMPDIR="$(mktemp -d)"
trap_add 'rm -rf $TMPDIR' EXIT
pip3 install --user pyasn1==0.4.7 pyasn1-modules==0.2.6 idna==2.8 ipaddress==1.0.22
pip3 install --user molecule==3.0.2
pip3 install --user ansible-lint yamllint
pip3 install --user docker==4.2.2 openshift jmespath
ansible-galaxy collection install community.kubernetes

setup_envs $tmp_sdk_root

header_text "Creating molecule sample"
go run ./hack/generate/samples/molecule/generate.go --path=$TMPDIR

pushd "$TMPDIR"
popd
cd $TMPDIR/memcached-molecule-operator

header_text "Running tests to check ansible molecule"
DEST_IMAGE="quay.io/example/ansible-test-operator:v0.0.1"
make docker-build IMG=$DEST_IMAGE
load_image_if_kind "$DEST_IMAGE"

header_text "Test Ansible Molecule scenarios"
make kustomize
if [ -f ./bin/kustomize ] ; then
  KUSTOMIZE="$(realpath ./bin/kustomize)"
else
  KUSTOMIZE="$(which kustomize)"
fi

KUSTOMIZE_PATH=${KUSTOMIZE}
KUSTOMIZE_PATH=${KUSTOMIZE} TEST_OPERATOR_NAMESPACE=default molecule test -s kind
OPERATOR_PULL_POLICY=Never OPERATOR_IMAGE=${DEST_IMAGE} TEST_CLUSTER_PORT=24443 TEST_OPERATOR_NAMESPACE=osdk-test molecule test --all
KUSTOMIZE_PATH=$KUSTOMIZE OPERATOR_PULL_POLICY=Never OPERATOR_IMAGE=${DEST_IMAGE} TEST_OPERATOR_NAMESPACE=osdk-test molecule test

