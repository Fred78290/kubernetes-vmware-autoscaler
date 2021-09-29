#!/bin/bash
CURDIR=$(dirname $0)
NODEGROUP_NAME="vmware-ca-k8s"
MASTERKUBE=${NODEGROUP_NAME}-masterkube
CONTROLNODES=3
WORKERNODES=3
FORCE=$1

# import govc hidden definitions
source ${CURDIR}/govc.defs

echo "Delete masterkube previous instance"

pushd $CURDIR/../

if [ "$(uname -s)" == "Linux" ]; then
    SED=sed
else
    SED=gsed
fi

if [ "$FORCE" = "YES" ]; then
    TOTALNODES=$((WORKERNODES + $CONTROLNODES))

    for NODEINDEX in $(seq 0 $TOTALNODES)
    do
        if [ $NODEINDEX = 0 ]; then
            MASTERKUBE_NODE="${MASTERKUBE}"
        elif [[ $NODEINDEX > $CONTROLNODES ]]; then
            NODEINDEX=$((NODEINDEX - $CONTROLNODES))
            MASTERKUBE_NODE="${NODEGROUP_NAME}-worker-0${NODEINDEX}"
        else
            MASTERKUBE_NODE="${NODEGROUP_NAME}-master-0${NODEINDEX}"
        fi

        if [ "$(govc vm.info ${MASTERKUBE_NODE} 2>&1)" ]; then
            echo "Delete VM: $MASTERKUBE_NODE"
            govc vm.power -persist-session=false -s $MASTERKUBE_NODE || echo "Already power off"
            govc vm.destroy $MASTERKUBE_NODE
        fi

        sudo $SED -i "/${MASTERKUBE_NODE}/d" /etc/hosts
    done
elif [ -f ./cluster/config ]; then
    for vm in $(kubectl get node -o json --kubeconfig ./cluster/config | jq '.items| .[] | .metadata.labels["kubernetes.io/hostname"]')
    do
        vm=$(echo -n $vm | tr -d '"')
        if [ ! -z "$(govc vm.info $vm 2>&1)" ]; then
            echo "Delete VM: $vm"
            govc vm.power -persist-session=false -s $vm
            govc vm.destroy $vm
        fi
        sudo $SED -i "/${vm}/d" /etc/hosts
    done

    if [ ! -z "$(govc vm.info $MASTERKUBE 2>&1)" ]; then
        echo "Delete VM: $MASTERKUBE"
        govc vm.power -persist-session=false -s $MASTERKUBE
        govc vm.destroy $MASTERKUBE
    fi
fi

./bin/kubeconfig-delete.sh $MASTERKUBE &> /dev/null

if [ -f config/vmware-autoscaler.pid ]; then
    kill $(cat config/vmware-autoscaler.pid)
fi

find cluster ! -name '*.md' -type f -exec rm -f "{}" "+"
find config ! -name '*.md' -type f -exec rm -f "{}" "+"

sudo $SED -i "/${MASTERKUBE}/d" /etc/hosts

popd