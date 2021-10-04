#!/bin/bash

set -e

SCHEME="vmware"
NODEGROUP_NAME="vmware-ca-k8s"
MASTERKUBE="${NODEGROUP_NAME}-masterkube"
CNI=flannel
NET_IF=$(ip route get 1|awk '{print $5;exit}')
KUBERNETES_VERSION=v1.21.0
CLUSTER_DIR=/etc/cluster
PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${HOSTNAME}"
HA_CLUSTER=false
CONTROL_PLANE_ENDPOINT=
CONTROL_PLANE_ENDPOINT_HOST=
CONTROL_PLANE_ENDPOINT_ADDR=
CLUSTER_NODES=
MAX_PODS=110
TOKEN_TLL="0s"
KUBEADM_TOKEN=$(kubeadm token generate)
APISERVER_ADVERTISE_PORT=6443
CLUSTER_DNS="10.96.0.10"
POD_NETWORK_CIDR="10.244.0.0/16"
SERVICE_NETWORK_CIDR="10.96.0.0/12"
LOAD_BALANCER_IP=
EXTERNAL_ETCD=false
NODEINDEX=0

TEMP=$(getopt -o i:g:h:c:k:n:p:x: --long node-index:,use-external-etcd:,load-balancer-ip:,node-group:,cluster-nodes:,control-plane-endpoint:,ha-cluster:,net-if:,provider-id:,cert-extra-sans:,cni:,kubernetes-version: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -g | --node-group)
        NODEGROUP_NAME="$2"
        shift 2
        ;;
    -i | --node-index)
        NODEINDEX="$2"
        shift 2
        ;;
    -c | --cni)
        CNI=$2
        shift 2
        ;;
    -h | --ha-cluster)
        HA_CLUSTER=$2
        shift 2
        ;;
    --load-balancer-ip)
        LOAD_BALANCER_IP="$2"
        shift 2
        ;;
    --control-plane-endpoint)
        CONTROL_PLANE_ENDPOINT="$2"
        IFS=: read CONTROL_PLANE_ENDPOINT_HOST CONTROL_PLANE_ENDPOINT_ADDR <<< $CONTROL_PLANE_ENDPOINT
        shift 2
        ;;
    --cluster-nodes)
        CLUSTER_NODES="$2"
        shift 2
        ;;
    -k | --kubernetes-version)
        KUBERNETES_VERSION="$2"
        shift 2
        ;;
    -n | --net-if)
        NET_IF=$2
        shift 2
        ;;
    -p | --provider-id)
        PROVIDERID=$2
        shift 2
        ;;
    -x | --cert-extra-sans)
        CERT_EXTRA_SANS=$2
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

# Check if interface exists, else take inet default gateway
ifconfig $NET_IF &> /dev/null || NET_IF=$(ip route get 1|awk '{print $5;exit}')
APISERVER_ADVERTISE_ADDRESS=$(ip addr show $NET_IF | grep "inet\s" | tr '/' ' ' | awk '{print $2}')
APISERVER_ADVERTISE_ADDRESS=$(echo $APISERVER_ADVERTISE_ADDRESS | awk '{print $1}')

if [ -z "$LOAD_BALANCER_IP" ]; then
    LOAD_BALANCER_IP=$APISERVER_ADVERTISE_ADDRESS
fi

mkdir -p $CLUSTER_DIR/etcd

echo -n "$LOAD_BALANCER_IP:6443" > $CLUSTER_DIR/manager-ip

sed -i "2i${APISERVER_ADVERTISE_ADDRESS} $(hostname)" /etc/hosts

if [ "x$KUBERNETES_VERSION" != "x" ]; then
    K8_OPTIONS="--token-ttl 0 --ignore-preflight-errors=All --apiserver-advertise-address $APISERVER_ADVERTISE_ADDRESS --kubernetes-version $KUBERNETES_VERSION"
else
    K8_OPTIONS="--token-ttl 0 --ignore-preflight-errors=All --apiserver-advertise-address $APISERVER_ADVERTISE_ADDRESS"
fi

if [ "x$CERT_EXTRA_SANS" != "x" ]; then
    K8_OPTIONS="$K8_OPTIONS --apiserver-cert-extra-sans=$CERT_EXTRA_SANS"
fi

if [ "x$CONTROL_PLANE_ENDPOINT" != "x" ]; then
    echo "$CONTROL_PLANE_ENDPOINT_ADDR   $CONTROL_PLANE_ENDPOINT_HOST" >> /etc/hosts
fi

if [ "$HA_CLUSTER" = "true" ]; then
    for CLUSTER_NODE in $(echo -n $CLUSTER_NODES | tr ',' ' ')
    do
        IFS=: read HOST IP <<< $CLUSTER_NODE
        sed -i "/$HOST/d" /etc/hosts
        echo "${IP}   ${HOST} ${HOST%%.*}" >> /etc/hosts
    done
fi

case "$CNI" in
    calico)
        K8_OPTIONS="$K8_OPTIONS --service-cidr 10.96.0.0/12 --pod-network-cidr 192.168.0.0/16"
        POD_NETWORK_CIDR="192.168.0.0/16"
        echo "Download calicoctl"

        curl -s -O -L https://github.com/projectcalico/calicoctl/releases/download/v3.1.0/calicoctl
        chmod +x calicoctl
        mv calicoctl /usr/local/bin
        ;;

    flannel)
        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr 10.244.0.0/16"
        POD_NETWORK_CIDR="10.244.0.0/16"
        ;;

    weave)
        ;;

    canal)
        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"
        POD_NETWORK_CIDR="10.244.0.0/16"
        ;;

    kube)
        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"
        POD_NETWORK_CIDR="10.244.0.0/16"
        ;;

    romana)
        ;;

    *)
        echo "CNI $CNI is not supported"
        exit -1
esac

if [ "$HA_CLUSTER" = "true" ]; then
    K8_OPTIONS="--ignore-preflight-errors=All --config=kubeadm-config.yaml"

cat > kubeadm-config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
bootstrapTokens:
- groups:
  - system:bootstrappers:kubeadm:default-node-token
  token: ${KUBEADM_TOKEN}
  ttl: ${TOKEN_TLL}
  usages:
  - signing
  - authentication
localAPIEndpoint:
  advertiseAddress: ${APISERVER_ADVERTISE_ADDRESS}
  bindPort: ${APISERVER_ADVERTISE_PORT}
nodeRegistration:
  criSocket: /var/run/crio/crio.sock
  name: ${NODENAME}
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
  kubeletExtraArgs:
    network-plugin: cni
    container-runtime: remote
    container-runtime-endpoint: /var/run/crio/crio.sock
    provider-id: ${PROVIDERID}
---
kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
clusterDNS:
- ${CLUSTER_DNS}
failSwapOn: false
hairpinMode: hairpin-veth
readOnlyPort: 10255
clusterDomain: cluster.local
cpuManagerReconcilePeriod: 0s
evictionPressureTransitionPeriod: 0s
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMinimumGCAge: 0s
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
rotateCertificates: true
runtimeRequestTimeout: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
maxPods: ${MAX_PODS}
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
certificatesDir: /etc/kubernetes/pki
clusterName: ${NODEGROUP_NAME}
dns:
  type: CoreDNS
imageRepository: k8s.gcr.io
kubernetesVersion: ${KUBERNETES_VERSION}
networking:
  dnsDomain: cluster.local
  serviceSubnet: ${SERVICE_NETWORK_CIDR}
  podSubnet: ${POD_NETWORK_CIDR}
scheduler: {}
controlPlaneEndpoint: ${CONTROL_PLANE_ENDPOINT_HOST}:${APISERVER_ADVERTISE_PORT}
apiServer:
  timeoutForControlPlane: 4m0s
  certSANs:
  - ${LOAD_BALANCER_IP}
  - ${CONTROL_PLANE_ENDPOINT_HOST}
  - ${CONTROL_PLANE_ENDPOINT_HOST%%.*}
EOF

    for CLUSTER_NODE in $(echo -n $CLUSTER_NODES | tr ',' ' ')
    do
        IFS=: read HOST IP <<< $CLUSTER_NODE
        cat >> kubeadm-config.yaml <<EOF
  - ${IP}
  - ${HOST}
  - ${HOST%%.*}
EOF
    done

    # External ETCD
    if [ "$EXTERNAL_ETCD" = "true" ]; then
        cat >> kubeadm-config.yaml <<EOF
etcd:
  external:
    caFile: /etc/etcd/ssl/ca.pem
    certFile: /etc/etcd/ssl/etcd.pem
    keyFile: /etc/etcd/ssl/etcd-key.pem
    endpoints:
EOF

        for CLUSTER_NODE in $(echo -n $CLUSTER_NODES | tr ',' ' ')
        do
            IFS=: read HOST IP <<< $CLUSTER_NODE
            echo "    - https://${HOST}:2379" >> kubeadm-config.yaml
        done
    fi

fi

if [ ! -f /etc/kubernetes/kubelet.conf ]; then

    if [ -z "$CNI" ]; then
        CNI="calico"
    fi

    CNI=$(echo "$CNI" | tr '[:upper:]' '[:lower:]')

    export KUBECONFIG=/etc/kubernetes/admin.conf

    sysctl net.bridge.bridge-nf-call-iptables=1
    echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

    echo "Init K8 cluster with options:$K8_OPTIONS, PROVIDERID=${PROVIDERID}"

    kubeadm init $K8_OPTIONS 2>&1

    echo "Retrieve token infos"

    openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //' | tr -d '\n' > $CLUSTER_DIR/ca.cert
    kubeadm token list 2>&1 | grep "authentication,signing" | awk '{print $1}'  | tr -d '\n' > $CLUSTER_DIR/token 

    echo "Set local K8 environement"

    mkdir -p $HOME/.kube
    cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
    chown $(id -u):$(id -g) $HOME/.kube/config

    cp /etc/kubernetes/admin.conf $CLUSTER_DIR/config

    if [ "$HA_CLUSTER" = "true" ]; then
        mkdir -p $CLUSTER_DIR/kubernetes/pki/etcd

        cp /etc/kubernetes/pki/ca.crt $CLUSTER_DIR/kubernetes/pki
        cp /etc/kubernetes/pki/ca.key $CLUSTER_DIR/kubernetes/pki
        cp /etc/kubernetes/pki/sa.key $CLUSTER_DIR/kubernetes/pki
        cp /etc/kubernetes/pki/sa.pub $CLUSTER_DIR/kubernetes/pki
        cp /etc/kubernetes/pki/front-proxy-ca.crt $CLUSTER_DIR/kubernetes/pki
        cp /etc/kubernetes/pki/front-proxy-ca.key $CLUSTER_DIR/kubernetes/pki

        if [ "$EXTERNAL_ETCD" != "true" ]; then
            cp /etc/kubernetes/pki/etcd/ca.crt $CLUSTER_DIR/kubernetes/pki/etcd/ca.crt
            cp /etc/kubernetes/pki/etcd/ca.key $CLUSTER_DIR/kubernetes/pki/etcd/ca.key
        fi
    fi

    chmod -R uog+r $CLUSTER_DIR/*
    
cat > patch.yaml <<EOF
spec:
    providerID: '${PROVIDERID}'
EOF

    kubectl patch node ${HOSTNAME} --patch-file patch.yaml

    if [ "$HA_CLUSTER" = "false" ]; then
        echo "Allow master to host pod"
        kubectl taint nodes ${HOSTNAME} node-role.kubernetes.io/master- 2>&1
    fi

    kubectl label nodes ${HOSTNAME} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "master=true"

    kubectl annotate node ${HOSTNAME} \
        "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" \
        "cluster.autoscaler.nodegroup/node-index=${NODEINDEX}" \
        "cluster.autoscaler.nodegroup/autoprovision=false" \
        "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" \
        --overwrite

    if [ "$CNI" = "calico" ]; then

        echo "Install calico network"
        kubectl apply -f https://docs.projectcalico.org/v3.8/manifests/calico.yaml

        #kubectl apply -f https://docs.projectcalico.org/manifests/calico.yaml 2>&1
        #kubectl apply -f https://docs.projectcalico.org/manifests/calico-etcd.yaml 2>&1

        curl -o /usr/local/bin/kubectl-calico -O -L  "https://github.com/projectcalico/calicoctl/releases/download/v3.19.1/calicoctl"
        chmod + /usr/local/bin/kubectl-calico

    elif [ "$CNI" = "flannel" ]; then

        echo "Install flannel network"

        kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml 2>&1

    elif [ "$CNI" = "weave" ]; then

        echo "Install weave network for K8"

        kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')" 2>&1

    elif [ "$CNI" = "canal" ]; then

        echo "Install canal network"

        kubectl apply -f https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/rbac.yaml 2>&1
        kubectl apply -f https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/canal.yaml 2>&1

    elif [ "$CNI" = "kube" ]; then

        echo "Install kube network"

        kubectl apply -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter.yaml 2>&1
        kubectl apply -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter-all-features.yaml 2>&1

    elif [ "$CNI" = "romana" ]; then

        echo "Install romana network"

        kubectl apply -f https://raw.githubusercontent.com/romana/romana/master/containerize/specs/romana-kubeadm.yml 2>&1

    fi

    echo "Done k8s master node"
else
    echo "Already installed k8s master node"
fi

exit 0