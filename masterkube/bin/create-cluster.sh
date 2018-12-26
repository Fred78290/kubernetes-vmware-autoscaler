#!/bin/bash

CNI=flannel
NET_IF==$(ip route get 1|awk '{print $5;exit}')
KUBERNETES_VERION=
DRY_RUN=false
CLUSTER_DIR=/etc/cluster
PROVIDERID=

[ -z "$1" ] || CNI=$1
[ -z "$2" ] || NET_IF=$2
[ -z "$3" ] || KUBERNETES_VERION=$3
[ -z "$4" ] || PROVIDERID=$4

# Check if interface exists, else take inet default gateway
ifconfig $NET_IF &> /dev/null || NET_IF=$(ip route get 1|awk '{print $5;exit}')
IPADDR=$(ip addr show $NET_IF | grep "inet\s" | tr '/' ' ' | awk '{print $2}')

mkdir -p $CLUSTER_DIR

echo -n "$IPADDR:6443" > $CLUSTER_DIR/manager-ip

if [ "x$KUBERNETES_VERION" != "x" ]; then
    K8_OPTIONS="--token-ttl 0 --ignore-preflight-errors=All --apiserver-advertise-address $IPADDR --kubernetes-version $KUBERNETES_VERION"
else
    K8_OPTIONS="--token-ttl 0 --ignore-preflight-errors=All --apiserver-advertise-address $IPADDR"
fi

if [ ! -f /etc/kubernetes/kubelet.conf ]; then

    if [ -z "$(grep 'provider-id' /etc/default/kubelet)" ]; then
        echo "KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true --provider-id=${PROVIDERID}'" > /etc/default/kubelet
        systemctl restart kubelet
    fi

    if [ -z "$CNI" ]; then
        CNI="calico"
    fi

    CNI=$(echo "$CNI" | tr '[:upper:]' '[:lower:]')

    export KUBECONFIG=/etc/kubernetes/admin.conf

    if [ "$CNI" = "calico" ]; then

        K8_OPTIONS="$K8_OPTIONS --service-cidr 10.96.0.0/12 --pod-network-cidr 192.168.0.0/16"

        echo "Download calicoctl"

        curl -s -O -L https://github.com/projectcalico/calicoctl/releases/download/v3.1.0/calicoctl
        chmod +x calicoctl
        mv calicoctl /usr/local/bin

    elif [ "$CNI" = "flannel" ]; then

        sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr 10.244.0.0/16"

    elif [ "$CNI" = "weave" ]; then

        sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

    elif [ "$CNI" = "canal" ]; then

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"

    elif [ "$CNI" = "canal" ]; then

        sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"

    elif [ "$CNI" = "kube" ]; then

        sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"

    elif [ "$CNI" = "romana" ]; then

        sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

    else
        echo "CNI $CNI is not supported"

        exit -1
    fi

    echo "Init K8 cluster with options:$K8_OPTIONS"

    kubeadm init $K8_OPTIONS

    echo "Retrieve token infos"

    openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //' | tr -d '\n' > $CLUSTER_DIR/ca.cert
    kubeadm token list | grep "authentication,signing" | awk '{print $1}'  | tr -d '\n' > $CLUSTER_DIR/token

    echo "Set local K8 environement"

    mkdir -p $HOME/.kube
    cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
    chown $(id -u):$(id -g) $HOME/.kube/config

    cp /etc/kubernetes/admin.conf $CLUSTER_DIR/config

    chmod +r $CLUSTER_DIR/*
    
    echo "Allow master to host pod"
    kubectl taint nodes --all node-role.kubernetes.io/master-

    if [ "$CNI" = "calico" ]; then

        echo "Install calico network"

        kubectl apply --dry-run=$DRY_RUN -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/etcd.yaml
        
        kubectl apply --dry-run=$DRY_RUN -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/rbac.yaml

        kubectl apply --dry-run=$DRY_RUN -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/calico.yaml

        kubectl apply --dry-run=$DRY_RUN -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calicoctl.yaml

    elif [ "$CNI" = "flannel" ]; then

        echo "Install flannel network"

        kubectl create -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml --dry-run=$DRY_RUN

    elif [ "$CNI" = "weave" ]; then

        echo "Install weave network for K8"

        kubectl apply --dry-run=$DRY_RUN -f "https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')"

    elif [ "$CNI" = "canal" ]; then

        echo "Install canal network"

        kubectl apply --dry-run=$DRY_RUN -f https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/rbac.yaml
        kubectl apply --dry-run=$DRY_RUN -f https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/canal.yaml

    elif [ "$CNI" = "kube" ]; then

        echo "Install kube network"

        kubectl apply --dry-run=$DRY_RUN -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter.yaml
        kubectl apply --dry-run=$DRY_RUN -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter-all-features.yaml

    elif [ "$CNI" = "romana" ]; then

        echo "Install romana network"

        kubectl apply --dry-run=$DRY_RUN -f https://raw.githubusercontent.com/romana/romana/master/containerize/specs/romana-kubeadm.yml

    fi

    echo "Done k8s master node"
else
    echo "Already installed k8s master node"
fi
