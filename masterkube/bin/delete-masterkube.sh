#!/bin/bash
CURDIR=$(dirname $0)
NODEGROUP_NAME="vmware-ca-k8s"
MASTERKUBE=${NODEGROUP_NAME}-masterkube

# import govc hidden definitions
source ${CURDIR}/govc.defs

echo "Delete masterkube previous instance"

pushd $CURDIR/../

if [ -f ./cluster/config ]; then
    for vm in $(kubectl get node -o json --kubeconfig ./cluster/config | jq '.items| .[] | .metadata.labels["kubernetes.io/hostname"]')
    do
        vm=$(echo -n $vm | tr -d '"')
        if [ ! -z "$(govc vm.info $vm 2>&1)" ]; then
            echo "Delete VM: $vm"
            govc vm.power -persist-session=false -s $vm
            govc vm.destroy $vm
        fi
    done
fi

./bin/kubeconfig-delete.sh $MASTERKUBE &> /dev/null

if [ -f config/vmware-autoscaler.pid ]; then
    kill $(cat config/vmware-autoscaler.pid)
fi

find cluster ! -name '*.md' -type f -exec rm -f "{}" "+"
find config ! -name '*.md' -type f -exec rm -f "{}" "+"

if [ "$(uname -s)" == "Linux" ]; then
    sudo sed -i "/${MASTERKUBE}/d" /etc/hosts
else
    sudo gsed -i -E "/${MASTERKUBE}/d" /etc/hosts
fi

popd