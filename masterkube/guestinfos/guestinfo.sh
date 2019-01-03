#/bin/bash

CURDIR=$(dirname $0)
VMHOME=~/vmware
SOURCEVMX="$VMHOME/cloud-init-guestinfo/cloud-init-guestinfo.vmx"
VMX="$VMHOME/clone-cloud-init-guestinfo/clone-cloud-init-guestinfo.vmx"
TZ=$(cat /etc/timezone)
SSHKEY=$(cat ~/.ssh/id_rsa.pub)
WHOAMI=$(whoami)
PASSWORD=$(uuidgen)

ls -l $VMX

if [ ! -f $VMX ]; then
	echo "Clone VM $SOURCEVMX"
	mkdir -p $(dirname $VMX)
	vmrun clone $SOURCEVMX $VMX full -cloneName="Test VMWareGuestInfo datasource"
elif [ ! -z "$(vmrun list | grep $VMX)" ]; then
	echo "Stop VM $VMX"
	vmrun stop $VMX
fi

cat > ${CURDIR}/userdata.yaml <<EOF
group:
    - kubernetes
runcmd:
    - 'KUBERNETES_VERSION="$(curl -sSL https://dl.k8s.io/release/stable.txt)"'
    - 'CNI_VERSION="v0.7.1"'
    - 'mkdir -p /opt/cni/bin'
    - 'mkdir -p /usr/local/bin'
    - 'curl https://get.docker.com | bash'
    - 'curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz'
    - 'cd /usr/local/bin'
    - 'curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}'
    - 'chmod +x /usr/local/bin/kube*'
    - 'echo "KUBELET_EXTRA_ARGS=''--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true''" > /etc/default/kubelet'
    - 'curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/kubelet.service" | sed ''s:/usr/bin:/usr/local/bin:g'' > /etc/systemd/system/kubelet.service'
    - 'mkdir -p /etc/systemd/system/kubelet.service.d'
    - 'curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/10-kubeadm.conf" | sed ''s:/usr/bin:/usr/local/bin:g'' > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf'
    - 'systemctl enable kubelet'
    - 'systemctl restart kubelet'
    - 'echo ''export PATH=/usr/local/bin:/opt/cni/bin:$PATH'' >> /etc/profile.d/apps-bin-path.sh'
    - 'apt autoremove -y'
    - '/usr/local/bin/kubeadm config images pull --kubernetes-version=${KUBERNETES_VERSION}'
users:
    -
        groups:
            - adm
            - users
        lock_passwd: false
        name: kubernetes
        passwd: $PASSWORD
        primary_group: kubernetes
        shell: /bin/bash
        ssh_authorized_keys:
            - "$SSHKEY"
        sudo: 'ALL=(ALL) NOPASSWD:ALL'
EOF

cat > ${CURDIR}/network.yaml <<EOF
network:
    version: 2
    ethernets:
        ens33:
            dhcp4: true
EOF

cat > $CURDIR/vendordata.yaml <<EOF
package_update: true
package_upgrade: true
timezone: $TZ
ssh_authorized_keys:
    - $SSHKEY
users:
    - default
system_info:
    default_user:
        name: $WHOAMI
EOF

cat > $CURDIR/metadata.json <<EOF
{
	"network": "$(cat ${CURDIR}/network.yaml | gzip -c9 | base64 -w 0)",
	"network.encoding": "gzip+base64",
	"local-hostname": "test-cloudinit-guestinfos",
	"instance-id": "test-cloudinit-guestinfos"
}
EOF

if [ -f "${VMX}.org" ]; then
	cp "${VMX}.org" "${VMX}"
else
	cp "${VMX}" "${VMX}.org"
fi

cat <<EOF | tee "${CURDIR}/guestinfo.txt" >> "$VMX"
guestinfo.metadata="$(cat ${CURDIR}/metadata.json | gzip -c9 | base64 -w 0)"
guestinfo.metadata.encoding="gzip+base64"
guestinfo.userdata="$(cat ${CURDIR}/userdata.yaml | gzip -c9 | base64 -w 0)"
guestinfo.userdata.encoding="gzip+base64"
guestinfo.vendordata="$(cat ${CURDIR}/vendordata.yaml | gzip -c9 | base64 -w 0)"
guestinfo.vendordata.encoding="gzip+base64"
EOF

#vmrun start "$VMX"
