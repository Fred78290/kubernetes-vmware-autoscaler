#/bin/bash
pushd $(dirname $0)

make container

[ $(uname -s) = "Darwin" ] && GOOS=darwin || GOOS=linux

./out/AutoScaler-autoscaler-$GOOS-amd64 \
    --config=masterkube/config/kubernetes-vmware-autoscaler.json \
    --save=masterkube/config/autoscaler-state.json \
    -v=9 \
    -logtostderr=true