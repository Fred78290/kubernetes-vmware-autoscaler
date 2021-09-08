#!/bin/bash
KUBERNETES_VERSION=$1
CNI_VERSION="v0.9.1"
ARCH=amd64
CRIO_VERSION=$(echo -n $KUBERNETES_VERSION | tr -d 'v' | tr '.' ' ' | awk '{ print $1"."$2 }')

if [ "$(uname -m)" == "aarch64" ]; then
    ARCH=arm64
fi

if [ "x$KUBERNETES_VERSION" == "x" ]; then
	RELEASE="v1.22.1"
else
	RELEASE=$KUBERNETES_VERSION
fi

cat <<SHELL > /etc/modules-load.d/crio.conf
overlay
br_netfilter
SHELL

cat >> /etc/sysctl.d/kubernetes.conf <<SHELL
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
SHELL

echo '1' > /proc/sys/net/bridge/bridge-nf-call-iptables

sysctl --system

. /etc/os-release

OS=x${NAME}_${VERSION_ID}

cat > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list <<SHELL
deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/$OS/ /
SHELL

cat > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable:cri-o:$CRIO_VERSION.list <<SHELL
deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/$CRIO_VERSION/$OS/ /
SHELL

curl -s -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/$OS/Release.key | sudo apt-key --keyring /etc/apt/trusted.gpg.d/libcontainers.gpg add -
curl -s -L https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable:cri-o:$CRIO_VERSION/$OS/Release.key | sudo apt-key --keyring /etc/apt/trusted.gpg.d/libcontainers-cri-o.gpg add -

apt-get update
apt-get dist-upgrade -y
apt-get autoremove -y
apt-get install -y jq socat conntrack net-tools traceroute podman cri-o cri-o-runc

mkdir -p /etc/crio/crio.conf.d/

cat > /etc/crio/crio.conf.d/02-cgroup-manager.conf <<SHELL
conmon_cgroup = "pod"
cgroup_manager = "cgroupfs"
SHELL

systemctl daemon-reload
systemctl enable crio --now

alias docker=podman

echo "Prepare kubernetes version $RELEASE"

mkdir -p /opt/cni/bin
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz

mkdir -p /usr/local/bin
cd /usr/local/bin
curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${RELEASE}/bin/linux/${ARCH}/{kubeadm,kubelet,kubectl,kube-proxy}
chmod +x /usr/local/bin/kube*

curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRIO_VERSION}.0/crictl-${CRIO_VERSION}.0-linux-${ARCH}.tar.gz  | tar -C /usr/local/bin -xz
chmod +x /usr/local/bin/crictl

echo "KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255'" > /etc/default/kubelet

cat > /etc/systemd/system/kubelet.service <<SHELL
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/

[Service]
ExecStart=/usr/local/bin/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
SHELL

mkdir -p /etc/systemd/system/kubelet.service.d

cat > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf <<"SHELL"
# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generate at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=/usr/local/bin/kubelet \$KUBELET_KUBECONFIG_ARGS \$KUBELET_CONFIG_ARGS \$KUBELET_KUBEADM_ARGS \$KUBELET_EXTRA_ARGS
SHELL

systemctl enable kubelet

echo 'export PATH=/opt/cni/bin:$PATH' >> /etc/bash.bashrc

kubeadm config images pull --kubernetes-version=$RELEASE
