#!/bin/bash
CURDIR=$(dirname $0)

pushd $CURDIR/../

export K8NAMESPACE=kube-system
export ETC_DIR=./config/deployment/metrics-server
export KUBERNETES_TEMPLATE=./templates/metrics-server

if [ -z "$DOMAIN_NAME" ]; then
    export DOMAIN_NAME=$(openssl x509 -noout -fingerprint -text < ./etc/ssl/cert.pem | grep 'Subject: CN =' | awk '{print $4}' | sed 's/\*\.//g')
echo "domain:$DOMAIN_NAME"
fi

mkdir -p $ETC_DIR

function deploy {
    echo "Create $ETC_DIR/$1.json"
    mkdir -p $(dirname $ETC_DIR/$1.json)
echo $(eval "cat <<EOF
$(<$KUBERNETES_TEMPLATE/$1.json)
EOF") | jq . > $ETC_DIR/$1.json

kubectl apply -f $ETC_DIR/$1.json --kubeconfig=./cluster/config
}

deploy clusterrole
deploy clusterrolebinding
deploy rolebinding
deploy apiservice
deploy serviceaccount
deploy deployment
deploy service
deploy system/clusterrole
deploy system/clusterrolebinding