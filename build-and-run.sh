#/bin/bash
pushd $(dirname $0)

make container
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
./out/$GOOS/$GOARCH/vsphere-autoscaler \
    --config=masterkube/config/kubernetes-vmware-autoscaler.json \
    --save=masterkube/config/autoscaler-state.json \
    --log-level=info
