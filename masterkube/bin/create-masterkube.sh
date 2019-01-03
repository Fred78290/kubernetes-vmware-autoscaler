#/bin/bash

# This script create every thing to deploy a simple kubernetes autoscaled cluster with AutoScaler.
# It will generate:
# Custom AutoScaler image with every thing for kubernetes
# Config file to deploy the cluster autoscaler.

CURDIR=$(dirname $0)

export MASTERKUBE="afp-bionic-masterkube"
export SSH_KEY=$(cat ~/.ssh/id_rsa.pub)
export KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
export KUBERNETES_PASSWORD=$(uuidgen)
export KUBECONFIG=$HOME/.kube/config
export TARGET_IMAGE=afp-bionic-k8s-$KUBERNETES_VERSION
export CNI_VERSION="v0.7.1"
export PROVIDERID="vmware://ca-grpc-vmware/object?type=node&name=$MASTERKUBE"
export MINNODES=0
export MAXNODES=5
export MAXTOTALNODES=$MAXNODES
export CORESTOTAL="0:16"
export MEMORYTOTAL="0:24"
export MAXAUTOPROVISIONNEDNODEGROUPCOUNT="1"
export SCALEDOWNENABLED="true"
export SCALEDOWNDELAYAFTERADD="1m"
export SCALEDOWNDELAYAFTERDELETE="1m"
export SCALEDOWNDELAYAFTERFAILURE="1m"
export SCALEDOWNUNEEDEDTIME="1m"
export SCALEDOWNUNREADYTIME="1m"
export DEFAULT_MACHINE="medium"
export UNREMOVABLENODERECHECKTIMEOUT="1m"
export OSDISTRO=$(uname -a)
export TRANSPORT="tcp"

if [ "$OSDISTRO" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    BASE64="base64 -w 0"
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    BASE64="base64"
fi

TEMP=$(getopt -o i:k:n:p:t:v: --long transport:,no-custom-image,image:,ssh-key:,cni-version:,password:,kubernetes-version:,max-nodes-total:,cores-total:,memory-total:,max-autoprovisioned-node-group-count:,scale-down-enabled:,scale-down-delay-after-add:,scale-down-delay-after-delete:,scale-down-delay-after-failure:,scale-down-unneeded-time:,scale-down-unready-time:,unremovable-node-recheck-timeout: -n "$0" -- "$@")

eval set -- "$TEMP"

# extract options and their arguments into variables.
while true; do
	case "$1" in
	-d | --default-machine)
		DEFAULT_MACHINE="$2"
		shift 2
		;;
	-i | --image)
		TARGET_IMAGE="$2"
		shift 2
		;;
	-k | --ssh-key)
		SSH_KEY="$2"
		shift 2
		;;
	-n | --cni-version)
		CNI_VERSION="$2"
		shift 2
		;;
	-p | --password)
		KUBERNETES_PASSWORD="$2"
		shift 2
		;;
	-t | --transport)
		TRANSPORT="$2"
		shift 2
		;;
	-v | --kubernetes-version)
		KUBERNETES_VERSION="$2"
        TARGET_IMAGE=afp-bionic-k8s-$KUBERNETES_VERSION
		shift 2
		;;
	--max-nodes-total)
		MAXTOTALNODES="$2"
		shift 2
		;;
	--cores-total)
		CORESTOTAL="$2"
		shift 2
		;;
	--memory-total)
		MEMORYTOTAL="$2"
		shift 2
		;;
	--max-autoprovisioned-node-group-count)
		MAXAUTOPROVISIONNEDNODEGROUPCOUNT="$2"
		shift 2
		;;
	--scale-down-enabled)
		SCALEDOWNENABLED="$2"
		shift 2
		;;
	--scale-down-delay-after-add)
		SCALEDOWNDELAYAFTERADD="$2"
		shift 2
		;;
	--scale-down-delay-after-delete)
		SCALEDOWNDELAYAFTERDELETE="$2"
		shift 2
		;;
	--scale-down-delay-after-failure)
		SCALEDOWNDELAYAFTERFAILURE="$2"
		shift 2
		;;
	--scale-down-unneeded-time)
		SCALEDOWNUNEEDEDTIME="$2"
		shift 2
		;;
	--scale-down-unready-time)
		SCALEDOWNUNREADYTIME="$2"
		shift 2
		;;
	--unremovable-node-recheck-timeout)
		UNREMOVABLENODERECHECKTIMEOUT="$2"
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

# GRPC network endpoint
if [ "$TRANSPORT" == "unix" ]; then
    LISTEN="/var/run/cluster-autoscaler/vmware.sock"
    CONNECTTO="/var/run/cluster-autoscaler/vmware.sock"
elif [ "$TRANSPORT" == "tcp" ]; then
    if [ "$OSDISTRO" == "Linux" ]; then
        NET_IF=$(ip route get 1 | awk '{print $5;exit}')
        IPADDR=$(ip addr show $NET_IF | grep "inet\s" | tr '/' ' ' | awk '{print $2}')
    else
        NET_IF=$(route get 1 | grep interface | awk '{print $2}')
        IPADDR=$(ifconfig $NET_IF | grep "inet\s" | sed -n 1p | awk '{print $2}')
    fi

    LISTEN="${IPADDR}:5200"
    CONNECTTO="${IPADDR}:5200"
else
    echo "Unknown transport: ${TRANSPORT}, should be unix or tcp"
    exit -1
fi

echo "Transport set to:${TRANSPORT}, listen endpoint at ${LISTEN}"

MOUNTPOINTS=$(cat <<EOF
    "$PWD/config": "/etc/cluster-autoscaler"
EOF
)

PACKAGE_UPGRADE="true"

RUN_CMD=$(
cat <<EOF
[
    "curl https://get.docker.com | bash",
    "mkdir -p /opt/cni/bin",
    "curl -L https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz | tar -C /opt/cni/bin -xz",
    "mkdir -p /opt/bin",
    "cd /opt/bin ; curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}",
    "chmod +x /opt/bin/kube*",
    "echo \"KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true'\" > /etc/default/kubelet",
    "curl -sSL \"https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/kubelet.service\" | sed 's:/usr/bin:/opt/bin:g' > /etc/systemd/system/kubelet.service",
    "mkdir -p /etc/systemd/system/kubelet.service.d",
    "curl -sSL \"https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/10-kubeadm.conf\" | sed 's:/usr/bin:/opt/bin:g' > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf",
    "ln -s /opt/bin/kubeadm /usr/local/bin/kubeadm",
    "ln -s /opt/bin/kubelet /usr/local/bin/kubelet",
    "ln -s /opt/bin/kubectl /usr/local/bin/kubectl",
    "systemctl enable kubelet",
    "systemctl restart kubelet",
    "echo 'export PATH=/opt/bin:/opt/cni/bin:\$PATH' >> /etc/profile.d/apps-bin-path.sh",
    "apt autoremove -y"
    "echo '#!/bin/bash' > /usr/local/bin/kubeimage",
    "echo '/opt/bin/kubeadm config images pull --kubernetes-version=${KUBERNETES_VERSION}' >> /usr/local/bin/kubeimage",
    "chmod +x /usr/local/bin/kubeimage"
]
EOF
)

KUBERNETES_USER=$(
	cat <<EOF
[
    {
        "name": "kubernetes",
        "primary_group": "kubernetes",
        "groups": [
            "adm",
            "users"
        ],
        "lock_passwd": false,
        "passwd": "$KUBERNETES_PASSWORD",
        "sudo": "ALL=(ALL) NOPASSWD:ALL",
        "shell": "/bin/bash",
        "ssh_authorized_keys": [
            "$SSH_KEY"
        ]
    }
]
EOF
)

MACHINE_DEFS=$(
	cat <<EOF
{
    "tiny": {
        "memsize": 2048,
        "vcpus": 2,
        "disksize": 5120
    },
    "medium": {
        "memsize": 4096,
        "vcpus": 2,
        "disksize": 10240
    },
    "large": {
        "memsize": 8192,
        "vcpus": 4,
        "disksize": 20480
    },
    "extra-large": {
        "memsize": 16384,
        "vcpus": 8,
        "disksize": 51200
    }
}
EOF
)

pushd $CURDIR/../

[ -d config ] || mkdir -p config
[ -d cluster ] || mkdir -p cluster
[ -d kubernetes ] || mkdir -p kubernetes

export PATH=$CURDIR:$PATH

if [ ! -f ./etc/ssl/privkey.pem ]; then
	mkdir -p ./etc/ssl/
	openssl genrsa 2048 >./etc/ssl/privkey.pem
	openssl req -new -x509 -nodes -sha1 -days 3650 -key ./etc/ssl/privkey.pem >./etc/ssl/cert.pem
	cat ./etc/ssl/cert.pem ./etc/ssl/privkey.pem >./etc/ssl/fullchain.pem
	chmod 644 ./etc/ssl/*
fi

export DOMAIN_NAME=$(openssl x509 -noout -fingerprint -text <./etc/ssl/cert.pem | grep 'Subject: CN' | tr '=' ' ' | awk '{print $3}' | sed 's/\*\.//g')

# Because AutoScaler on MacOS doesn't support local image, we can't use custom image
POWERSTATE=$(
    cat <<EOF
    , "power_state": {
        "mode": "reboot",
        "message": "Reboot VM due upgrade",
        "condition": true
    }
EOF
)

if [ -z "$(govc vm.info)" ]; then
    echo "Create vmware preconfigured image"

    create-image.sh --password=$KUBERNETES_PASSWORD \
        --cni-version=$CNI_VERSION \
        --custom-image=$TARGET_IMAGE \
        --kubernetes-version=$KUBERNETES_VERSION
fi

./bin/delete-masterkube.sh

echo "Launch custom $MASTERKUBE instance with $TARGET_IMAGE"

cat > ./config/network.yaml <<EOF
network:
    version: 2
    ethernets:
        eth0:
            dhcp4: true
        eth1:
            dhcp4: true
EOF

cat > ./config/vendordata.yaml <<EOF
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

cat > ./config/metadata.json <<EOF
{
	"network": "$(cat ./config/network.yaml | gzip -c9 | $BASE64)",
	"network.encoding": "gzip+base64",
	"local-hostname": "$MASTERKUBE",
	"instance-id": "$MASTERKUBE"
}
EOF

cat <<EOF | tee ./config/userdata.json | python2 -c "import json,sys,yaml; print yaml.safe_dump(json.load(sys.stdin), width=500, indent=4, default_flow_style=False)" >./config/userdata.yaml
{
}
EOF

METADATA=$(gzip -c9 <./config/metadata.yaml | $BASE64)
USERDATA=$(gzip -c9 <./config/userdata.yaml | $BASE64)
VENDORDATA=$(gzip -c9 <./config/vendordata.yaml | $BASE64)

govc vm.clone -link=false -on=false -c=2 -m=4096 -vm=$TARGET_IMAGE $MASTERKUBE

govc vm.change -vm "${MASTERKUBE}" -e guestinfo.metadata="${METADATA}"
govc vm.change -vm "${MASTERKUBE}" -e guestinfo.metadata.encoding="gzip+base64"
govc vm.change -vm "${MASTERKUBE}" -e guestinfo.userdata="${USERDATA}"
govc vm.change -vm "${MASTERKUBE}" -e guestinfo.userdata.encoding="gzip+base64"
govc vm.change -vm "${MASTERKUBE}" -e guestinfo.vendordata="${VENDORDATA}"
govc vm.change -vm "${MASTERKUBE}" -e guestinfo.vendordata.encoding="gzip+base64"

govc vm.power -on "${MASTERKUBE}"
IPADDR=$(govc vm.ip -wait 5m "${MASTERKUBE}")

AutoScaler mount $PWD/bin masterkube:/masterkube/bin
AutoScaler mount $PWD/templates masterkube:/masterkube/templates
AutoScaler mount $PWD/etc masterkube:/masterkube/etc
AutoScaler mount $PWD/cluster masterkube:/etc/cluster
AutoScaler mount $PWD/kubernetes masterkube:/etc/kubernetes
AutoScaler mount $PWD/config masterkube:/etc/cluster-autoscaler

echo "Prepare masterkube instance"
scp ./bin/* kubernetes@${IPADDR}:/usr/local/bin/*

echo "Start kubernetes masterkube instance master node"
ssh kubernetes@${IPADDR} sudo bash -c "export PATH=/opt/cni/bin:/masterkube/bin:\$PATH; create-cluster.sh flannel eth0 '$KUBERNETES_VERSION' '$PROVIDERID'" 

scp kubernetes@${IPADDR}:/etc/cluster/* ./cluster

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

kubectl annotate node masterkube "cluster.autoscaler.nodegroup/name=ca-grpc-AutoScaler" "cluster.autoscaler.nodegroup/node-index=0" "cluster.autoscaler.nodegroup/autoprovision=false" "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" --overwrite --kubeconfig=./cluster/config
kubectl label nodes masterkube "cluster.autoscaler.nodegroup/name=ca-grpc-AutoScaler" "master=true" --overwrite --kubeconfig=./cluster/config
kubectl create secret tls kube-system -n kube-system --key ./etc/ssl/privkey.pem --cert ./etc/ssl/fullchain.pem --kubeconfig=./cluster/config

./bin/kubeconfig-merge.sh masterkube cluster/config

echo "Write AutoScaler cloud autoscaler provider config"

echo $(eval "cat <<EOF
$(<./templates/cluster/grpc-config.json)
EOF") | jq . >./config/grpc-config.json

cat <<EOF | jq . > config/kubernetes-vmware-autoscaler.json
{
    "network": "$TRANSPORT",
    "listen": "$LISTEN",
    "secret": "AutoScaler",
    "minNode": $MINNODES,
    "maxNode": $MAXNODES,
    "nodePrice": 0.0,
    "podPrice": 0.0,
    "image": "$LAUNCH_IMAGE_URL",
    "vm-provision": true,
    "kubeconfig": "$KUBECONFIG",
    "optionals": {
        "pricing": false,
        "getAvailableMachineTypes": false,
        "newNodeGroup": false,
        "templateNodeInfo": false,
        "createNodeGroup": false,
        "deleteNodeGroup": false
    },
    "kubeadm": {
        "address": "$MASTER_IP",
        "token": "$TOKEN",
        "ca": "sha256:$CACERT",
        "extras-args": [
            "--ignore-preflight-errors=All"
        ]
    },
    "default-machine": "$DEFAULT_MACHINE",
    "machines": $MACHINE_DEFS,
    "cloud-init": {
        "package_update": false,
        "package_upgrade": false,
        "users": $KUBERNETES_USER,
        "runcmd": [
            "kubeadm config images pull --kubernetes-version=${KUBERNETES_VERION}"
        ],
        "ssh_authorized_keys": [
            "$SSH_KEY"
        ],
        "group": [
            "kubernetes"
        ]
    },
    "mount-point": {
        $MOUNTPOINTS
    }
}
EOF

HOSTS_DEF=$(AutoScaler info masterkube | grep IPv4 | awk "{print \$2 \"    masterkube.$DOMAIN_NAME masterkube-dashboard.$DOMAIN_NAME\"}")

if [ "$OSDISTRO" == "Linux" ]; then
	sudo sed -i '/masterkube/d' /etc/hosts
	sudo bash -c "echo '$HOSTS_DEF' >> /etc/hosts"
else
	sudo sed -i '' '/masterkube/d' /etc/hosts
	sudo bash -c "echo '$HOSTS_DEF' >> /etc/hosts"
fi

./bin/create-ingress-controller.sh
./bin/create-dashboard.sh
#./bin/create-autoscaler.sh
./bin/create-helloworld.sh

popd
