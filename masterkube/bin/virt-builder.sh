#!/bin/bash

TARGET_IMAGE=$HOME/.local/AutoScaler/cache/builder-kubernetes-amd64.img
KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
KUBERNETES_PASSWORD=$(uuidgen)
CNI_VERSION=v0.7.1

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
apt-get install curl cloud-init jq socat -y

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

systemctl enable kubelet

echo 'export PATH=/opt/cni/bin:\$PATH' >> /etc/profile.d/apps-bin-path.sh
EOF

chmod +x /tmp/prepare-k8s-bionic.sh

sudo virt-builder ubuntu-18.04 \
    -v --format raw \
    --copy-in /run/systemd/resolve/resolv.conf:/etc \
    --root-password password:$KUBERNETES_PASSWORD \
    --copy-in /tmp/$INIT_SCRIPT:/usr/local/bin \
    --run-command /usr/local/bin/prepare-k8s-bionic.sh \
    --firstboot-command "dpkg-reconfigure openssh-server" \
    -o $TARGET_IMAGE

#sudo virt-sysprep -a $TARGET_IMAGE

rm /tmp/prepare-k8s-bionic.sh

echo "Created image $TARGET_IMAGE with kubernetes version $KUBERNETES_VERSION"
