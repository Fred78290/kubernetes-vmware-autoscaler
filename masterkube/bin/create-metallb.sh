#!/bin/bash
CURDIR=$(dirname $0)

pushd $CURDIR/../

export ETC_DIR=./config/deployment/metallb

mkdir -p $ETC_DIR

cat > $ETC_DIR/config.yml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - $METALLB_IP_RANGE
EOF

kubectl --kubeconfig=./cluster/config apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.6/manifests/namespace.yaml
kubectl --kubeconfig=./cluster/config create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
kubectl --kubeconfig=./cluster/config apply -f $ETC_DIR/config.yml
kubectl --kubeconfig=./cluster/config apply -f https://raw.githubusercontent.com/google/metallb/v0.9.6/manifests/metallb.yaml
