#/bin/bash

CURDIR=$(dirname $0)
SSHKEY=$(cat ~/.ssh/id_rsa.pub)
WHOAMI=$(whoami)
PASSWORD=$(uuidgen)

if [ $(uname -s) == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    BASE64="base64 -w 0"
    VMHOME=~/vmware
    SOURCEVMX="$VMHOME/cloud-init-guestinfo/cloud-init-guestinfo.vmx"
    VMX="$VMHOME/clone-cloud-init-guestinfo/clone-cloud-init-guestinfo.vmx"
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    BASE64=base64
    VMHOME=~/Documents/Virtual\ Machines.localized
    SOURCEVMX="$VMHOME/afp-bionic-minimal.vmwarevm/afp-bionic-minimal.vmx"
    VMX="$VMHOME/clone-afp-bionic-minimal.vmwarevm/clone-afp-bionic-minimal.vmx"
fi

cat > ${CURDIR}/userdata.yaml <<EOF
#cloud-config
group:
    - kubernetes
runcmd:
    - KUBERNETES_VERSION=v1.17.5
    - CNI_VERSION=v0.8.5
    - mkdir -p /opt/cni/bin
    - mkdir -p /usr/local/bin
    - curl https://get.docker.com | bash
    - curl -L "https://github.com/containernetworking/plugins/releases/download/\${CNI_VERSION}/cni-plugins-amd64-\${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz
    - cd /usr/local/bin
    - curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/\${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}
    - chmod +x /usr/local/bin/kube*
    - echo "KUBELET_EXTRA_ARGS=--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true" > /etc/default/kubelet
    - curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/\${KUBERNETES_VERSION}/build/debs/kubelet.service" | sed 's:/usr/bin:/usr/local/bin:g > /etc/systemd/system/kubelet.service
    - mkdir -p /etc/systemd/system/kubelet.service.d
    - curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/\${KUBERNETES_VERSION}/build/debs/10-kubeadm.conf" | sed 's:/usr/bin:/usr/local/bin:g > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
    - systemctl enable kubelet
    - systemctl restart kubelet
    - echo 'export PATH=/usr/local/bin:/opt/cni/bin:\$PATH' >> /etc/profile.d/apps-bin-path.sh
    - apt autoremove -y
    - /usr/local/bin/kubeadm config images pull --kubernetes-version=\${KUBERNETES_VERSION}
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
            - $SSHKEY
        sudo: ALL=(ALL) NOPASSWD:ALL
EOF

cat > ${CURDIR}/userdata.yaml <<EOF
EOF

cat > ${CURDIR}/network.yaml <<EOF
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
      set-name: eth0
      match:
        macaddress: 00:50:56:38:2A:26
    eth1:
      set-name: eth1
      match:
        macaddress: 00:50:56:3F:78:A5
      gateway4: 10.0.0.1
      addresses:
      - 10.0.0.70/24
      nameservers:
        search:
        - aldune.com
        addresses:
        - 10.0.0.1
EOF

cat > $CURDIR/vendordata.yaml <<EOF
#cloud-config
package_update: true
package_upgrade: true
timezone: $TZ
ssh_authorized_keys:
    - $SSHKEY
users:
    - default
system_info:
    default_user:
        name: kubernetes
EOF

cat > $CURDIR/metadata.json <<EOF
{
    "network": "$(cat ${CURDIR}/network.yaml | gzip -c9 | $BASE64)",
    "network.encoding": "gzip+base64",
    "local-hostname": "test-cloudinit-guestinfos",
    "instance-id": "test-cloudinit-guestinfos"
}
EOF

cat > $CURDIR/xmetadata.json <<EOF
{
    "local-hostname": "test-cloudinit-guestinfos",
    "instance-id": "test-cloudinit-guestinfos"
}
EOF


cat <<EOF > "${CURDIR}/guestinfo.txt"
ethernet0.addressType = "static"
ethernet0.connectionType = "nat"
ethernet0.address = "00:50:56:38:2A:26"
ethernet0.pciSlotNumber = "33"
ethernet0.present = "TRUE"
ethernet0.virtualDev = "vmxnet3"
ethernet1.addressType = "static"
ethernet1.address = "00:50:56:3F:78:A5"
ethernet1.pciSlotNumber = "34"
ethernet1.present = "TRUE"
ethernet1.virtualDev = "vmxnet3"
guestinfo.metadata="$(cat ${CURDIR}/metadata.json | gzip -c9 | ${BASE64})"
guestinfo.metadata.encoding="gzip+base64"
guestinfo.userdata="$(cat ${CURDIR}/userdata.yaml | gzip -c9 | ${BASE64})"
guestinfo.userdata.encoding="gzip+base64"
guestinfo.vendordata="$(cat ${CURDIR}/vendordata.yaml | gzip -c9 | ${BASE64})"
guestinfo.vendordata.encoding="gzip+base64"
EOF

if [ ! -f "$VMX" ]; then
	echo "Clone VM $SOURCEVMX"
	mkdir -p $(dirname "$VMX")
elif [ ! -z "$(vmrun list | grep $VMX)" ]; then
	echo "Stop VM $VMX"
	vmrun stop $VMX
fi

rm -rf $(dirname "$VMX")

vmrun clone "$SOURCEVMX" "$VMX" full -cloneName="Test VMWareGuestInfo datasource"

cp "${VMX}" "${VMX}.org"

cat "${VMX}.org" | sed \
    -e "/ethernet0/d" \
    -e "/ethernet1/d"  > "${VMX}"

cat "${CURDIR}/guestinfo.txt" >> "${VMX}"

vmrun start "$VMX"
