#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source ./hack/lib/test_lib.sh
source ./hack/lib/image_lib.sh

# install SDK binaries
make install

# ansible proxy test require a running cluster; run during e2e instead
go test -count=1 ./internal/ansible/proxy/...

# create test directories
test_dir=./test
tests=$test_dir/e2e-ansible

export TRACE=1
export GO111MODULE=on

# Install pre-requirements for Molecule
pip3 install --user pyasn1==0.4.7 pyasn1-modules==0.2.6 idna==2.8 ipaddress==1.0.22
pip3 install --user molecule==3.0.2
pip3 install --user ansible-lint yamllint
pip3 install --user docker==4.2.2 openshift jmespath
ansible-galaxy collection install community.kubernetes

# set default envvars
setup_envs $tmp_sdk_root

go test $tests -v -ginkgo.v
