#!/bin/bash
CURDIR=$(dirname $0)

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

./bin/kubeconfig-delete.sh masterkube &> /dev/null

if [ -f config/vmware-autoscaler.pid ]; then
    kill $(cat config/vmware-autoscaler.pid)
fi

find cluster ! -name '*.md' -type f -exec rm -f "{}" "+"
find config ! -name '*.md' -type f -exec rm -f "{}" "+"
find kubernetes ! -name '*.md' -type f -exec rm -f "{}" "+"

popd