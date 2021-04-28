#/bin/bash
pushd $(dirname $0)

make container
GOARCH=$(go env GOARCH)
./out/vsphere-autoscaler-$GOARCH \
    --config=masterkube/config/kubernetes-vmware-autoscaler.json \
    --save=masterkube/config/autoscaler-state.json \
    --log-level=info
