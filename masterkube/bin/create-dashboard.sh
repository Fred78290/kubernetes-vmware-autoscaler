#!/bin/bash

echo "Deploy kubernetes dashboard"

# This file is intent to deploy dashboard inside the masterkube
CURDIR=$(dirname $0)

pushd $CURDIR/../

export K8NAMESPACE=kubernetes-dashboard
export ETC_DIR=./config/deployment/dashboard
export KUBERNETES_TEMPLATE=./templates/dashboard
export SUBPATH_POD_NAME='$(POD_NAME)'
export REWRITE_TARGET='/$1'

if [ -z "$DOMAIN_NAME" ]; then
    export DOMAIN_NAME=$(openssl x509 -noout -subject -in ./etc/ssl/cert.pem | awk -F= '{print $NF}' | sed -e 's/^[ \t]*//' | sed 's/\*\.//g')
fi

mkdir -p $ETC_DIR

function deploy {
    echo "Create $ETC_DIR/$1.json"
echo $(eval "cat <<EOF
$(<$KUBERNETES_TEMPLATE/$1.json)
EOF") | jq . > $ETC_DIR/$1.json

kubectl apply -f $ETC_DIR/$1.json --kubeconfig=./cluster/config
}

deploy namespace
deploy serviceaccount
deploy service

kubectl create secret tls $K8NAMESPACE \
    -n $K8NAMESPACE \
    --key ./etc/ssl/privkey.pem \
    --cert ./etc/ssl/fullchain.pem \
    --kubeconfig=./cluster/config

kubectl create secret generic kubernetes-dashboard-certs \
    --from-file=dashboard.key=./etc/ssl/privkey.pem \
    --from-file=dashboard.crt=./etc/ssl/fullchain.pem \
    --kubeconfig=./cluster/config \
    -n $K8NAMESPACE

deploy csrf
deploy keyholder
deploy settings

deploy role
deploy clusterrole
deploy rolebinding
deploy clusterrolebinding
deploy deployment
deploy ingress
deploy scrapersvc
deploy scraper

# Create the service account in the current namespace 
# (we assume default)
kubectl create serviceaccount my-dashboard-sa -n $K8NAMESPACE --kubeconfig=./cluster/config
# Give that service account root on the cluster
kubectl create clusterrolebinding my-dashboard-sa --clusterrole=cluster-admin --serviceaccount=$K8NAMESPACE:my-dashboard-sa --kubeconfig=./cluster/config
# Find the secret that was created to hold the token for the SA
kubectl get secrets -n $K8NAMESPACE --kubeconfig=./cluster/config
# Show the contents of the secret to extract the token
# kubectl describe secret my-dashboard-sa-token-xxxxx
DASHBOARD_TOKEN=$(kubectl  --kubeconfig=./cluster/config -n $K8NAMESPACE describe secret $(kubectl get secret -n $K8NAMESPACE  --kubeconfig=./cluster/config | awk '/^my-dashboard-sa-token-/{print $1}') | awk '$1=="token:"{print $2}')
echo "Dashboard token:$DASHBOARD_TOKEN"

echo $DASHBOARD_TOKEN > ./cluster/dashboard-token