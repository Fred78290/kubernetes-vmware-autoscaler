#!/bin/bash

CURDIR=$(dirname $0)

pushd $CURDIR/../

export K8NAMESPACE=ingress-nginx
export ETC_DIR=./config/deployment/ingress
export KUBERNETES_TEMPLATE=./templates/ingress

mkdir -p $ETC_DIR

function deploy {
    echo "Create $ETC_DIR/$1.json"
    mkdir -p $(dirname $ETC_DIR/$1)
echo $(eval "cat <<EOF
$(<$KUBERNETES_TEMPLATE/$1.json)
EOF") | jq . > $ETC_DIR/$1.json

kubectl apply -f $ETC_DIR/$1.json --kubeconfig=./cluster/config
}

deploy essentials/namespace
deploy essentials/clusterrole
deploy essentials/clusterrolebinding
deploy essentials/class
deploy essentials/tcp-services-configmap
deploy essentials/udp-services-configmap

deploy default-backend/deployement
deploy default-backend/service

deploy controller/serviceaccount
deploy controller/configmap
deploy controller/role
deploy controller/rolebinding
deploy controller/service-webhook
deploy controller/service
deploy controller/deployment

deploy admission-webhooks/validating-webhook
deploy admission-webhooks/job-patch/clusterrole
deploy admission-webhooks/job-patch/clusterrolebinding
deploy admission-webhooks/job-patch/job-createSecret
deploy admission-webhooks/job-patch/job-patchWebhook
deploy admission-webhooks/job-patch/role
deploy admission-webhooks/job-patch/rolebinding
deploy admission-webhooks/job-patch/serviceaccount

sleep 20

$CURDIR/wait-pod.sh ingress-nginx-controller $K8NAMESPACE
