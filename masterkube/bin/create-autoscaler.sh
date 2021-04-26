#/bin/bash
LAUNCH_CA=$1

CURDIR=$(dirname $0)

pushd $CURDIR/../

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

export K8NAMESPACE=kube-system
export ETC_DIR=./config/deployment/autoscaler
export KUBERNETES_TEMPLATE=./templates/autoscaler

mkdir -p $ETC_DIR

function deploy {
    echo "Create $ETC_DIR/$1.json"
echo $(eval "cat <<EOF
$(<$KUBERNETES_TEMPLATE/$1.json)
EOF") | jq . > $ETC_DIR/$1.json

kubectl apply -f $ETC_DIR/$1.json --kubeconfig=./cluster/config
}

deploy service-account-autoscaler
deploy service-account-vsphere
deploy cluster-role
deploy role
deploy cluster-role-binding
deploy role-binding

if [ "$LAUNCH_CA" == YES ]; then
    deploy deployment
elif [ "$LAUNCH_CA" == "DEBUG" ]; then
    deploy autoscaler
elif [ "$LAUNCH_CA" == "LOCAL" ]; then
    nohup ../out/vsphere-autoscaler-$(go env GOARCH) \
        --kubeconfig=$KUBECONFIG \
        --config=$PWD/config/kubernetes-vmware-autoscaler.json \
        --save=$PWD/config/vmware-autoscaler-state.json \
        -v=1 \
        -logtostderr=true  1>>config/vmware-autoscaler.log 2>&1 &
    pid="$!"

    echo -n "$pid" > config/vmware-autoscaler.pid

    deploy autoscaler
else
    deploy deployment
fi

popd
