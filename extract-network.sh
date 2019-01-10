#!/bin/bash

govc vm.info -json afp-slyo-k8s-autoscaled-test \
    | jq '.VirtualMachines[0].Config.ExtraConfig[] | select (.Key == "guestinfo.metadata")'.Value \
    | tr -d '"' \
    | base64 -D \
    | gzip -d \
    | jq .network | tr -d '"' \
    | base64 -D \
    | gzip -d \


#govc vm.info -json afp-slyo-k8s-autoscaled-test \
#    | jq '.VirtualMachines[0].Config.ExtraConfig[] | select (.Key == "guestinfo.userdata")'.Value \
#    | tr -d '"' \
#    | base64 -D \
#    | gzip -d