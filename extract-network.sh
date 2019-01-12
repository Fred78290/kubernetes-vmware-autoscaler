#!/bin/bash

if [ $(uname -s) == "Linux" ]; then
    DBASE64="base64 -d"
else
    DBASE64="base64 -D"
fi

govc vm.info -json afp-slyo-k8s-autoscaled-test \
    | jq '.VirtualMachines[0].Config.ExtraConfig[] | select (.Key == "guestinfo.metadata")'.Value \
    | tr -d '"' \
    | $DBASE64 \
    | gzip -d \
    | jq .network | tr -d '"' \
    | $DBASE64 \
    | gzip -d \


#govc vm.info -json afp-slyo-k8s-autoscaled-test \
#    | jq '.VirtualMachines[0].Config.ExtraConfig[] | select (.Key == "guestinfo.userdata")'.Value \
#    | tr -d '"' \
#    | base64 -D \
#    | gzip -d