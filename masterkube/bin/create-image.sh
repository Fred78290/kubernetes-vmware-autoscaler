#/bin/bash

# This script customize bionic-server-cloudimg-amd64.img to include docker+kubernetes
# Before running this script, you must install some elements with the command below
# sudo apt install qemu-kvm libvirt-clients libvirt-daemon-system bridge-utils virt-manager
# This process disable netplan and use old /etc/network/interfaces because I don't now why each VM instance running the customized image
# have the same IP with different mac address.

# /usr/lib/python3/dist-packages/cloudinit/net/netplan.py

TARGET_IMAGE=$HOME/.local/AutoScaler/cache/bionic-kubernetes-amd64.img
KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
KUBERNETES_PASSWORD=$(uuidgen)
CNI_VERSION=v0.7.1
CACHE=~/.local/AutoScaler/cache
TEMP=`getopt -o i:k:n:p:v: --long custom-image:,ssh-key:,cni-version:,password:,kubernetes-version: -n "$0" -- "$@"`
eval set -- "$TEMP"

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

# Hack because virt-customize doesn't recopy the good /etc/resolv.conf due dnsmasq
if [ -f /run/systemd/resolve/resolv.conf ]; then
    RESOLVCONF=/run/systemd/resolve/resolv.conf
else
    RESOLVCONF=/etc/resolv.conf
fi

# Grab nameserver/domainname
NAMESERVER=$(grep nameserver $RESOLVCONF | awk '{print $2}')
DOMAINNAME=$(grep search $RESOLVCONF | awk '{print $2}')
INIT_SCRIPT=prepare-k8s-bionic.sh

cat > /tmp/prepare-k8s-bionic.sh <<EOF
#/bin/bash

echo "nameserver $NAMESERVER" > /etc/resolv.conf 
echo "search $DOMAINNAME" >> /etc/resolv.conf 

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get upgrade -y
apt-get install jq socat -y

apt-get autoremove -y

curl https://get.docker.com | bash

mkdir -p /opt/cni/bin
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz

sed -i 's/PasswordAuthentication/#PasswordAuthentication/g' /etc/ssh/sshd_config 

mkdir -p /usr/local/bin
cd /usr/local/bin
curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}
chmod +x /usr/local/bin/kube*

curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/kubelet.service" | sed "s:/usr/bin:/usr/local/bin:g" > /etc/systemd/system/kubelet.service
mkdir -p /etc/systemd/system/kubelet.service.d
curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/10-kubeadm.conf" | sed "s:/usr/bin:/usr/local/bin:g" > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf

echo "KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true'" > /etc/default/kubelet

systemctl enable kubelet

echo 'export PATH=/opt/cni/bin:\$PATH' >> /etc/profile.d/apps-bin-path.sh
export PATH=/opt/cni/bin:/usr/local/bin:\$PATH

kubeadm config images pull --kubernetes-version=${KUBERNETES_VERION}

echo "network: {config: disabled}" > /etc/cloud/cloud.cfg.d/99-disable-network-config.cfg

cat > /etc/systemd/network/20-dhcp.network <<SHELL
[Match]
Name=en*

[Network]
DHCP=yes

[DHCP]
UseMTU=true
UseDomains=true
ClientIdentifier=mac
SHELL

echo "cloud-init	cloud-init/datasources	multiselect	NoCloud,None" | debconf-set-selections
dpkg-reconfigure cloud-init
EOF

chmod +x /tmp/prepare-k8s-bionic.sh

[ -d $CACHE ] || mkdir -p $CACHE

if [ ! -f $CACHE/bionic-server-cloudimg-amd64.img ]; then
    wget https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img -O $CACHE/bionic-server-cloudimg-amd64.img
fi

cp $CACHE/bionic-server-cloudimg-amd64.img $TARGET_IMAGE

qemu-img resize $TARGET_IMAGE 5G
sudo virt-customize --network -a $TARGET_IMAGE --timezone Europe/Paris
sudo virt-customize --network -a $TARGET_IMAGE --root-password password:$KUBERNETES_PASSWORD
sudo virt-customize --network -a $TARGET_IMAGE --copy-in /tmp/$INIT_SCRIPT:/usr/local/bin
sudo virt-customize --network -a $TARGET_IMAGE --run-command /usr/local/bin/$INIT_SCRIPT
sudo virt-customize --network -a $TARGET_IMAGE --firstboot-command "/bin/rm /etc/machine-id; /bin/systemd-machine-id-setup"
sudo virt-sysprep -a $TARGET_IMAGE

rm /tmp/prepare-k8s-bionic.sh

echo "Created image $TARGET_IMAGE with kubernetes version $KUBERNETES_VERSION"
