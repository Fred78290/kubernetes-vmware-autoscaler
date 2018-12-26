#!/bin/bash
KUBERNETES_VERSION=$1
CNI_VERSION="v0.7.1"

curl -s https://get.docker.com | bash

# On lxd container remove overlay mod test
if [ -f /lib/systemd/system/containerd.service ]; then
	sed -i  's/ExecStartPre=/#ExecStartPre=/g' /lib/systemd/system/containerd.service
	systemctl daemon-reload
	systemctl restart containerd.service
	systemctl restart docker
fi

cat >> /etc/dhcp/dhclient.conf << EOF
interface "eth0" {
}
EOF

if [ "x$KUBERNETES_VERSION" == "x" ]; then
	RELEASE="$(curl -sSL https://dl.k8s.io/release/stable.txt)"
else
	RELEASE=$KUBERNETES_VERSION
fi

echo "Prepare kubernetes version $RELEASE"

mkdir -p /opt/cni/bin
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz

mkdir -p /usr/local/bin
cd /usr/local/bin
curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${RELEASE}/bin/linux/amd64/{kubeadm,kubelet,kubectl}
chmod +x {kubeadm,kubelet,kubectl}

if [ -f /run/systemd/resolve/resolv.conf ]; then
	echo "KUBELET_EXTRA_ARGS='--resolv-conf=/run/systemd/resolve/resolv.conf --fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true'" > /etc/default/kubelet
else
	echo "KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true'" > /etc/default/kubelet
fi

curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${RELEASE}/build/debs/kubelet.service" | sed "s:/usr/bin:/usr/local/bin:g" > /etc/systemd/system/kubelet.service
mkdir -p /etc/systemd/system/kubelet.service.d
curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${RELEASE}/build/debs/10-kubeadm.conf" | sed "s:/usr/bin:/usr/local/bin:g" > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf

systemctl enable kubelet && systemctl restart kubelet

# Clean all image
for img in $(docker images --format "{{.Repository}}:{{.Tag}}")
do
	echo "Delete docker image:$img"
	docker rmi $img
done

echo 'export PATH=/opt/cni/bin:$PATH' >> /etc/bash.bashrc
#echo 'export PATH=/usr/local/bin:/opt/cni/bin:$PATH' >> /etc/profile.d/apps-bin-path.sh

kubeadm config images pull --kubernetes-version=$RELEASE
