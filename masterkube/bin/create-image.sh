#/bin/bash

set -e

# This script customize bionic-server-cloudimg-amd64.img to include docker+kubernetes
# Before running this script, you must install some elements with the command below
# sudo apt install qemu-kvm libvirt-clients libvirt-daemon-system bridge-utils virt-manager
# This process disable netplan and use old /etc/network/interfaces because I don't now why each VM instance running the customized image
# have the same IP with different mac address.

# /usr/lib/python3/dist-packages/cloudinit/net/netplan.py

KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
KUBERNETES_PASSWORD=$(uuidgen)
CNI_VERSION=v0.7.1
SSH_KEY=$(cat ~/.ssh/id_rsa.pub)
CACHE=~/.local/vmware/cache
ISODIR=~/.local/vmware/cache/iso
TARGET_IMAGE=afp-bionic-kubernetes-$KUBERNETES_VERSION
PASSWORD=$(uuidgen)
OSDISTRO=$(uname -a)
TEMP=`getopt -o i:k:n:p:v: --long custom-image:,ssh-key:,cni-version:,password:,kubernetes-version: -n "$0" -- "$@"`

eval set -- "$TEMP"

if [ "$OSDISTRO" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    BASE64='base64 -w 0'
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    BASE64=base64
fi

mkdir -p $ISODIR

pushd $CACHE

# extract options and their arguments into variables.
while true ; do
	#echo "1:$1"
    case "$1" in
        -i|--custom-image) TARGET_IMAGE="$2" ; shift 2;;
        -k|--ssh-key) SSH_KEY=$2 ; shift 2;;
        -n|--cni-version) CNI_VERSION=$2 ; shift 2;;
        -p|--password) KUBERNETES_PASSWORD=$2 ; shift 2;;
        -v|--kubernetes-version) KUBERNETES_VERSION=$2 ; shift 2;;
        --) shift ; break ;;
        *) echo "$1 - Internal error!" ; exit 1 ;;
    esac
done

cat > ./iso/user-data <<EOF
runcmd:
    - sed -i 'd/^GRUB_CMDLINE_LINUX="/GRUB_CMDLINE_LINUX="ipv6.disable=1 net.ifnames=0 biosdevname=0/g' /etc/default/grub
    - update-grub2
    - mkdir -p /opt/cni/bin
    - mkdir -p /usr/local/bin
    - curl https://get.docker.com | bash
    - curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz
    - cd /usr/local/bin
    - curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}
    - chmod +x /usr/local/bin/kube*
    - echo 'KUBELET_EXTRA_ARGS="--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true"' > /etc/default/kubelet
    - curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/kubelet.service" | sed 's:/usr/bin:/usr/local/bin:g' > /etc/systemd/system/kubelet.service
    - mkdir -p /etc/systemd/system/kubelet.service.d
    - curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/10-kubeadm.conf" | sed 's:/usr/bin:/usr/local/bin:g' > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
    - systemctl enable kubelet
    - systemctl restart kubelet
    - echo 'export PATH=/opt/cni/bin:\$PATH' >> /etc/profile.d/apps-bin-path.sh
    - apt autoremove -y
    - /usr/local/bin/kubeadm config images pull --kubernetes-version=${KUBERNETES_VERSION}
    - CLOUD_INIT_SOURCES=\$(python3 -c "import os; from cloudinit import sources; print(os.path.dirname(sources.__file__));" 2>/dev/null || (exit_code="\${?}"; echo "failed to find python runtime" 1>&2; exit "\${exit_code}"; ))
    - [ -z "\${CLOUD_INIT_SOURCES}" ] && echo "cloud-init not found" 1>&2 && exit 1
    - echo "Cloud init sources located here: \$CLOUD_INIT_SOURCES"
    - curl -sSL -o "\${CLOUD_INIT_SOURCES}/DataSourceVMwareGuestInfo.py" "https://raw.githubusercontent.com/akutz/cloud-init-vmware-guestinfo/master/DataSourceVMwareGuestInfo.py"
    - mkdir -p /etc/cloud/cloud.cfg.d
    - curl -sSL -o /etc/cloud/cloud.cfg.d/99-DataSourceVMwareGuestInfo.cfg "https://raw.githubusercontent.com/akutz/cloud-init-vmware-guestinfo/master/99-DataSourceVMwareGuestInfo.cfg"
    - sed -i 's/None/None, VMwareGuestInfo/g' /etc/cloud/cloud.cfg.d/90_dpkg.cfg
    - rm /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg
    - cloud-init clean
    - rm /var/log/cloud-ini*
EOF

cat > ./iso/network.yaml <<EOF
network:
    version: 2
    ethernets:
        ens33:
            dhcp4: true
EOF

cat > ./iso/vendor-data <<EOF
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

cat > ./iso/meta-data <<EOF
{
	"network": "$(cat ./iso/network.yaml | gzip -c9 | $BASE64)",
	"network.encoding": "gzip+base64",
	"local-hostname": "$TARGET_IMAGE",
	"instance-id": "$TARGET_IMAGE"
}
EOF

METADATA=$(cat ./iso/meta-data | $BASE64)
USERDATA=$(cat ./iso/user-data | $BASE64)
VENDORDATA=$(cat ./iso/vendor-data | $BASE64)

if [ "$OSDISTRO" == "Linux" ]; then
    genisoimage -output cidata.iso -volid cidata -joliet -rock user-data meta-data vendor-data
else
    mkisofs -V cidata -J -r -o cidata.iso iso
fi

if [ ! -f bionic-server-cloudimg-amd64.ova ]; then
    wget https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.ova -O bionic-server-cloudimg-amd64.ova
    NETWORK=$(govc import.spec bionic-server-cloudimg-amd64.ova | jq .NetworkMapping[0].Name)
    govc import.spec bionic-server-cloudimg-amd64.ova \
        | jq --arg GOVC_NETWORK $GOVC_NETWORK --arg NETWORK 'VM Network' '.NetworkMapping |= [ { Name: $NETWORK, Network: $GOVC_NETWORK } ]' \
        > bionic-server-cloudimg-amd64.spec
fi

cat bionic-server-cloudimg-amd64.spec \
    | jq --arg SSH_KEY "$SSH_KEY" \
        --arg SSH_KEY "$SSH_KEY" \
        --arg USERDATA "$VENDORDATA" \
        --arg PASSWORD "$PASSWORD" \
        --arg TARGET_IMAGE "$TARGET_IMAGE" '.Name = $TARGET_IMAGE | .PropertyMapping |= [ { Key: "instance-id", Value: $TARGET_IMAGE }, { Key: "hostname", Value: $TARGET_IMAGE }, { Key: "public-keys", Value: $SSH_KEY }, { Key: "user-data", Value: $USERDATA }, { Key: "password", Value: $PASSWORD } ]' \
        > bionic-server-cloudimg-amd64.txt

echo "Import OVA to $TARGET_IMAGE"
#govc import.ova \
    -options=bionic-server-cloudimg-amd64.txt \
    -dump=true \
    -debug=true \
    -folder=/$GOVC_DATACENTER/vm/$GOVC_FOLDER \
    -ds=/$GOVC_DATACENTER/datastore/$GOVC_CLUSTER/CUSTOMER/$GOVC_FOLDER/$GOVC_DATASTORE \
    -name=$TARGET_IMAGE bionic-server-cloudimg-amd64.ova

ovftool \
    --acceptAllEulas \
    --name=$TARGET_IMAGE \
    --datastore=$GOVC_DATASTORE \
    --vmFolder=$GOVC_FOLDER \
    --diskMode=thin \
    --prop:instance-id="$TARGET_IMAGE" \
    --prop:hostname="$TARGET_IMAGE" \
    --prop:public-keys="$SSH_KEY" \
    --prop:user-data="$VENDORDATA" \
    --prop:password="$PASSWORD" \
    --net:"VM Network"="$GOVC_NETWORK" \
    https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.ova \
    vi://$GOVC_USERNAME:$GOVC_PASSWORD@$GOVC_HOST/$GOVC_RESOURCE_POOL

exit 
echo "Import cidata.iso to $TARGET_IMAGE"
govc datastore.upload  cidata.iso $TARGET_IMAGE/cidata.iso

CDROM=$(govc device.ls -json -vm $TARGET_IMAGE|grep cdrom|awk '{print $1;exit}')

if [ -z "$(govc device.ls -json -vm $TARGET_IMAGE|grep cdrom)" ]; then
    IDE=$(govc device.ls -vm $TARGET_IMAGE | grep ide- | awk '{print $1;exit}')
    govc device.cdrom.add -vm $TARGET_IMAGE -controller $IDE
    CDROM=$(govc device.ls -json -vm $TARGET_IMAGE|grep cdrom|awk '{print $1;exit}')
fi

echo "Connect CD-rom to $TARGET_IMAGE"
govc device.connect -vm $TARGET_IMAGE $CDROM
govc device.cdrom.insert -vm $TARGET_IMAGE -device $CDROM $TARGET_IMAGE/cidata.iso

echo "Created image $TARGET_IMAGE with kubernetes version $KUBERNETES_VERSION"

popd
