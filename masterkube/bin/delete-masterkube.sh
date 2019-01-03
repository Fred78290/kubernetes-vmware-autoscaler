#!/bin/bash
CURDIR=$(dirname $0)

echo "Delete masterkube previous instance"

pushd $CURDIR/../

if [ -f ./cluster/config ]; then
    for vm in $(kubectl get node -o json --kubeconfig ./cluster/config | jq '.items| .[] | .metadata.labels["kubernetes.io/hostname"]')
    do
        vm=$(echo -n $vm | tr -d '"')
        echo "Delete VM: $vm"
        UUID=$(govc vm.info $vm | grep UUID | awk '{print $2}')
        if [ ! -z "$UUID" ]; then
            govc vm.power -off $vm
            govc vm.destroy -vm.uuid=$UUID
        fi
    done
fi

./bin/kubeconfig-delete.sh masterkube &> /dev/null

if [ -f config/vmware-autoscaler.pid ]; then
    kill $(cat config/vmware-autoscaler.pid)
fi

rm -rf cluster/*
rm -rf config/*
rm -rf kubernetes/*

popd