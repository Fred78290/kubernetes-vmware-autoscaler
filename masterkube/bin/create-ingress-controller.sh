#!/bin/bash

CURDIR=$(dirname $0)

pushd $CURDIR/../

export K8NAMESPACE=ingress-nginx
export ETC_DIR=./config/deployment/ingress
export KUBERNETES_TEMPLATE=./templates/ingress

mkdir -p $ETC_DIR

sed "s/__K8NAMESPACE__/$K8NAMESPACE/g" $KUBERNETES_TEMPLATE/deploy.yaml > $ETC_DIR/deploy.yaml

kubectl apply -f $ETC_DIR/deploy.yaml --kubeconfig=./cluster/config

echo -n "Wait for ingress controller availability"

while [ -z "$(kubectl --kubeconfig=./cluster/config get po -n $K8NAMESPACE 2>/dev/null | grep 'ingress-nginx-controller')" ];
do
    sleep 1
    echo -n "."
done

echo

kubectl wait --kubeconfig=./cluster/config --namespace $K8NAMESPACE --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=120s