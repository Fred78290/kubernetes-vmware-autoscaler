#!/bin/bash

CURDIR=$(dirname $0)

pushd $CURDIR/../

export K8NAMESPACE=ingress-nginx
export ETC_DIR=./config/deployment/ingress
export KUBERNETES_TEMPLATE=./templates/ingress

mkdir -p $ETC_DIR

function deploy {
    echo "Create $ETC_DIR/$1.json"
echo $(eval "cat <<EOF
$(<$KUBERNETES_TEMPLATE/$1.json)
EOF") | jq . > $ETC_DIR/$1.json

kubectl apply -f $ETC_DIR/$1.json --kubeconfig=./cluster/config
}

deploy namespace
deploy default-backend-deployement
deploy default-backend-service
deploy configmap
deploy tcp-services-configmap
deploy udp-services-configmap
deploy rbac-service-account
deploy rbac-cluster-role
deploy rbac-role
deploy rbac-role-binding
deploy rbac-cluster-role-binding
deploy deployment
deploy service
