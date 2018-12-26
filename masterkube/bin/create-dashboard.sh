#!/bin/bash

# This file is intent to deploy dashboard inside the masterkube
CURDIR=$(dirname $0)

pushd $CURDIR/../

export K8NAMESPACE=kube-system
export ETC_DIR=./config/deployment/dashboard
export KUBERNETES_TEMPLATE=./templates/dashboard

if [ -z "$DOMAIN_NAME" ]; then
    export DOMAIN_NAME=$(openssl x509 -noout -fingerprint -text < ./etc/ssl/cert.pem | grep 'Subject: CN' | tr '=' ' ' | awk '{print $3}' | sed 's/\*\.//g')
fi

mkdir -p $ETC_DIR

function deploy {
    echo "Create $ETC_DIR/$1.json"
echo $(eval "cat <<EOF
$(<$KUBERNETES_TEMPLATE/$1.json)
EOF") | jq . > $ETC_DIR/$1.json

kubectl apply -f $ETC_DIR/$1.json --kubeconfig=./cluster/config
}

kubectl create secret generic kubernetes-dashboard-certs \
    --from-file=dashboard.key=./etc/ssl/privkey.pem \
    --from-file=dashboard.crt=./etc/ssl/fullchain.pem \
    --kubeconfig=./cluster/config \
    -n $K8NAMESPACE

deploy serviceaccount
deploy role
deploy rolebinding
deploy deployment
deploy service
deploy ingress

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