#!/bin/bash
NODEGROUP_NAME="vmware-ca-k8s"
MASTERKUBE="${NODEGROUP_NAME}-masterkube"
CLUSTER_DIR=/etc/cluster
APISERVER_ADVERTISE_PORT=6443
CONTROL_PLANE_ENDPOINT=
CONTROL_PLANE_ENDPOINT_HOST=
CONTROL_PLANE_ENDPOINT_ADDR=
CLUSTERNET_IF=

TEMP=$(getopt -o u:c:n: --long user:,cluster-nodes:,net-if: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -c|--cluster-nodes)
        CLUSTER_NODES="$2"
        shift 2
        ;;

    -n | --net-if)
        NET_IF=$2
        shift 2
        ;;

    -u | --user)
        USER=$2
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

# Check if interface exists, else take inet default gateway

ifconfig $NET_IF &> /dev/null || NET_IF=$(ip route get 1|awk '{print $5;exit}')
APISERVER_ADVERTISE_ADDRESS=$(ip addr show $NET_IF | grep "inet\s" | tr '/' ' ' | awk '{print $2}')
APISERVER_ADVERTISE_ADDRESS=$(echo $APISERVER_ADVERTISE_ADDRESS | awk '{print $1}')

ETCDIPS=()
ETCDHOSTS=()
ETCDNAMES=()

for CLUSTER_NODE in $(echo -n ${CLUSTER_NODES} | tr ',' ' ')
do
    IFS=: read HOST IP <<< ${CLUSTER_NODE}

    ETCDIPS+=($IP)
    ETCDHOSTS+=($HOST)
    ETCDNAMES+=(${HOST%%.*})
done

kubeadm init phase certs etcd-ca

mkdir -p /home/${USER}/cluster/kubernetes/pki
mkdir -p /home/${USER}/cluster/etcd

for INDEX in "${!ETCDHOSTS[@]}";
do
    echo "Generate etcd config index: $INDEX"

    IP=${ETCDIPS[$INDEX]}
    HOST=${ETCDHOSTS[$INDEX]}
    NAME=${ETCDNAMES[$INDEX]}
    ETCINDEX="0$((INDEX+1))"
    CONFIG=/home/${USER}/cluster/etcd/kubeadmcfg-${ETCINDEX}.yaml
    SERVICE=/home/${USER}/cluster/etcd/etcd-${ETCINDEX}.service

    cat > ${SERVICE} << EOF
[Unit]
Description=Etcd Server
After=network.target
After=network-online.target
Wants=network-online.target
Documentation=https://github.com/coreos
[Service]
Type=notify
WorkingDirectory=/var/lib/etcd/
ExecStart=/usr/local/bin/etcd \
    --advertise-client-urls=https://${IP}:2379 \
    --cert-file=/etc/kubernetes/pki/etcd/server.crt \
    --key-file=/etc/kubernetes/pki/etcd/server.key \
    --client-cert-auth=true \
    --data-dir=/var/lib/etcd \
    --initial-advertise-peer-urls=https://${IP}:2380 \
    --initial-cluster-state=new \
    --initial-cluster-token=etcd-cluster-0 \
    --initial-cluster=${ETCDNAMES[0]}=https://${ETCDIPS[0]}:2380,${ETCDNAMES[1]}=https://${ETCDIPS[1]}:2380,${ETCDNAMES[2]}=https://${ETCDIPS[2]}:2380 \
    --listen-client-urls=https://${IP}:2379,http://127.0.0.1:2379 \
    --listen-metrics-urls=http://127.0.0.1:2381 \
    --listen-peer-urls=https://${IP}:2380 \
    --name=${NAME} \
    --peer-client-cert-auth=true \
    --peer-cert-file=/etc/kubernetes/pki/etcd/peer.crt \
    --peer-key-file=/etc/kubernetes/pki/etcd/peer.key \
    --peer-trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt \
    --snapshot-count=10000 \
    --trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
EOF

    cat > ${CONFIG} << EOF
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: ${APISERVER_ADVERTISE_ADDRESS}
  bindPort: ${APISERVER_ADVERTISE_PORT}
---
apiVersion: "kubeadm.k8s.io/v1beta2"
kind: ClusterConfiguration
etcd:
    local:
        serverCertSANs:
        - ${ETCDHOSTS[0]}
        - ${ETCDHOSTS[1]}
        - ${ETCDHOSTS[2]}
        peerCertSANs:
        - ${ETCDIPS[0]}
        - ${ETCDIPS[1]}
        - ${ETCDIPS[2]}
        extraArgs:
            initial-cluster: ${ETCDNAMES[0]}=https://${ETCDIPS[0]}:2380,${ETCDNAMES[1]}=https://${ETCDIPS[1]}:2380,${ETCDNAMES[2]}=https://${ETCDIPS[2]}:2380
            initial-cluster-state: new
            name: ${NAME}
            listen-peer-urls: https://${IP}:2380
            listen-client-urls: https://${IP}:2379
            advertise-client-urls: https://${IP}:2379
            initial-advertise-peer-urls: https://${IP}:2380
EOF

    # Generate cert
    kubeadm init phase certs etcd-server --config=${CONFIG}
    kubeadm init phase certs etcd-peer --config=${CONFIG}
    kubeadm init phase certs etcd-healthcheck-client --config=${CONFIG}
    kubeadm init phase certs apiserver-etcd-client --config=${CONFIG}

    # Copy cert
    cp -R /etc/kubernetes/pki /home/${USER}/cluster/kubernetes/pki/${ETCINDEX}

    # Clean all except ca
    find /etc/kubernetes/pki -not -name ca.crt -not -name ca.key -type f -delete
done

# Remove ca.key
find /home/${USER}/cluster/kubernetes/pki/02 -name ca.key -type f -delete
find /home/${USER}/cluster/kubernetes/pki/03 -name ca.key -type f -delete

chmod -R uog+r /home/${USER}/cluster

# Create etcd daemon
#kubeadm init phase etcd local --config=/home/${USER}/cluster/kubernetes/kubeadmcfg-01.yaml
