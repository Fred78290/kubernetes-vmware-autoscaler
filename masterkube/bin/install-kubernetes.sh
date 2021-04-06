#!/bin/bash
KUBERNETES_VERSION=$1
CNI_VERSION="v0.9.1"

curl -s https://get.docker.com | bash

if [ "x$KUBERNETES_VERSION" == "x" ]; then
	RELEASE="v1.20.5"
else
	RELEASE=$KUBERNETES_VERSION
fi

echo "Prepare kubernetes version $RELEASE"

mkdir -p /opt/cni/bin
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz

mkdir -p /usr/local/bin
cd /usr/local/bin
curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${RELEASE}/bin/linux/amd64/{kubeadm,kubelet,kubectl,kube-proxy}
chmod +x /usr/local/bin/kube*

echo "KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255'" > /etc/default/kubelet

cat > /etc/systemd/system/kubelet.service <<SHELL
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/
After=docker.service
Requires=docker.service

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
