#!/bin/bash
SCHEME="vmware"
NODEGROUP_NAME="vmware-ca-k8s"
MASTERKUBE="${NODEGROUP_NAME}-masterkube"
CNI=flannel
CLUSTER_DIR=/etc/cluster
CONTROL_PLANE_ENDPOINT=
CONTROL_PLANE_ENDPOINT_HOST=
CONTROL_PLANE_ENDPOINT_ADDR=
CLUSTER_NODES=
HA_CLUSTER=
EXTERNAL_ETCD=NO
NODEINDEX=0

TEMP=$(getopt -o i:g:c:n:p: --long node-index:,use-external-etcd:,ha-cluster:,node-group:,cluster-nodes:,control-plane-endpoint:,provider-id:, -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -g|--node-group)
        NODEGROUP_NAME="$2"
        shift 2
        ;;
    -i|--node-index)
        NODEINDEX="$2"
        shift 2
        ;;
    -c|--control-plane-endpoint)
        CONTROL_PLANE_ENDPOINT="$2"
        IFS=: read CONTROL_PLANE_ENDPOINT_HOST CONTROL_PLANE_ENDPOINT_ADDR <<< $CONTROL_PLANE_ENDPOINT
        shift 2
        ;;
    -n|--cluster-nodes)
        CLUSTER_NODES="$2"
        shift 2
        ;;
    -p|--provider-id)
        PROVIDERID=$2
        shift 2
        ;;
    -h | --ha-cluster)
        HA_CLUSTER=$2
        shift 2
        ;;
    --use-external-etcd)
        EXTERNAL_ETCD=$2
        shift 2
        ;;
    --)
        shift
        break
        ;;

    *)
        echo "$1 - Internal error!"
        exit 1
        ;;
    esac
done

sed -i "/$CONTROL_PLANE_ENDPOINT_HOST/d" /etc/hosts
echo "$CONTROL_PLANE_ENDPOINT_ADDR   $CONTROL_PLANE_ENDPOINT_HOST" >> /etc/hosts

for CLUSTER_NODE in $(echo -n $CLUSTER_NODES | tr ',' ' ')
do
    IFS=: read HOST IP <<< $CLUSTER_NODE
    sed -i "/$HOST/d" /etc/hosts
    echo "$IP   $HOST" >> /etc/hosts
done

mkdir -p /etc/kubernetes/pki/etcd

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

cp cluster/config /etc/kubernetes/admin.conf

if [ "$HA_CLUSTER" = "true" ]; then
    cp cluster/kubernetes/pki/ca.crt /etc/kubernetes/pki
    cp cluster/kubernetes/pki/ca.key /etc/kubernetes/pki
    cp cluster/kubernetes/pki/sa.key /etc/kubernetes/pki
    cp cluster/kubernetes/pki/sa.pub /etc/kubernetes/pki
    cp cluster/kubernetes/pki/front-proxy-ca.key /etc/kubernetes/pki
    cp cluster/kubernetes/pki/front-proxy-ca.crt /etc/kubernetes/pki

    chown -R root:root /etc/kubernetes/pki

    chmod 600 /etc/kubernetes/pki/ca.crt
    chmod 600 /etc/kubernetes/pki/ca.key
    chmod 600 /etc/kubernetes/pki/sa.key
    chmod 600 /etc/kubernetes/pki/sa.pub
    chmod 600 /etc/kubernetes/pki/front-proxy-ca.key
    chmod 600 /etc/kubernetes/pki/front-proxy-ca.crt

    if [ -f cluster/kubernetes/pki/etcd/ca.crt ]; then
        cp cluster/kubernetes/pki/etcd/ca.crt /etc/kubernetes/pki/etcd
        cp cluster/kubernetes/pki/etcd/ca.key /etc/kubernetes/pki/etcd

        chmod 600 /etc/kubernetes/pki/etcd/ca.crt
        chmod 600 /etc/kubernetes/pki/etcd/ca.key
    fi

    kubeadm join ${MASTER_IP} \
        --token "${TOKEN}" \
        --discovery-token-ca-cert-hash "sha256:${CACERT}" \
        --control-plane
else
    kubeadm join ${MASTER_IP} \
        --token "${TOKEN}" \
        --discovery-token-ca-cert-hash "sha256:${CACERT}"
fi

export KUBECONFIG=/etc/kubernetes/admin.conf

cat > patch.yaml <<EOF
spec:
    providerID: '${PROVIDERID}'
EOF

kubectl patch node ${HOSTNAME} --patch-file patch.yaml

kubectl label nodes ${HOSTNAME} \
    "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" \
    "node-role.kubernetes.io/worker=" "worker=true" \
    --overwrite

kubectl annotate node ${HOSTNAME} \
    "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" \
    "cluster.autoscaler.nodegroup/node-index=${NODEINDEX}" \
    "cluster.autoscaler.nodegroup/autoprovision=false" \
    "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" \
    --overwrite
