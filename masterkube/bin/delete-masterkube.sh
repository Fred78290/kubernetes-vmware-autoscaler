#!/bin/bash
CURDIR=$(dirname $0)

echo "Delete masterkube previous instance"

pushd $CURDIR/../

if [ -f ./cluster/config ]; then
    for vm in $(kubectl get node -o json --kubeconfig ./cluster/config | jq '.items| .[] | .metadata.labels["kubernetes.io/hostname"]')
    do
        vm=$(echo -n $vm | tr -d '"')
        echo "Delete AutoScaler VM: $vm"
        AutoScaler delete $vm -p &> /dev/null
    done
fi

./bin/kubeconfig-delete.sh masterkube &> /dev/null

if [ -f config/AutoScaler-autoscaler.pid ]; then
    kill $(cat config/AutoScaler-autoscaler.pid)
fi

rm -rf cluster/*
rm -rf config/*
rm -rf kubernetes/*

popd