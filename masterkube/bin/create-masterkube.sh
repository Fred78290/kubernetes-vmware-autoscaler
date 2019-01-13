#/bin/bash

# This script create every thing to deploy a simple kubernetes autoscaled cluster with AutoScaler.
# It will generate:
# Custom AutoScaler image with every thing for kubernetes
# Config file to deploy the cluster autoscaler.

set -e

CURDIR=$(dirname $0)

export NODEGROUP_NAME="afp-slyo-ca-k8s"
export MASTERKUBE="${NODEGROUP_NAME}-masterkube"
export SSH_KEY=$(cat ~/.ssh/id_rsa.pub)
export KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
export KUBERNETES_PASSWORD=$(uuidgen)
export KUBECONFIG=$HOME/.kube/config
export ROOT_IMG_NAME=afp-slyo-bionic-kubernetes
export TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
export CNI_VERSION="v0.7.1"
export PROVIDERID="vmware://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
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
export OSDISTRO=$(uname -s)
export TRANSPORT="tcp"
export USER=ubuntu

if [ "$OSDISTRO" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    BASE64="base64 -w 0"
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    BASE64="base64"
fi

TEMP=$(getopt -o i:k:n:p:s:t: --long transport:,no-custom-image,image:,ssh-key:,cni-version:,password:,kubernetes-version:,max-nodes-total:,cores-total:,memory-total:,max-autoprovisioned-node-group-count:,scale-down-enabled:,scale-down-delay-after-add:,scale-down-delay-after-delete:,scale-down-delay-after-failure:,scale-down-unneeded-time:,scale-down-unready-time:,unremovable-node-recheck-timeout: -n "$0" -- "$@")

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
	-s | --ssh-key)
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
	-k | --kubernetes-version)
		KUBERNETES_VERSION="$2"
        TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
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
        "memsize": 4096,
        "vcpus": 2,
        "disksize": 10240
    },
    "medium": {
        "memsize": 8192,
        "vcpus": 4,
        "disksize": 20480
    },
    "large": {
        "memsize": 16384,
        "vcpus": 8,
        "disksize": 51200
    },
    "extra-large": {
        "memsize": 32767,
        "vcpus": 16,
        "disksize": 102400
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

if [ -z "$(govc vm.info $TARGET_IMAGE 2>&1)" ]; then
    echo "Create vmware preconfigured image"

    create-image.sh --password=$KUBERNETES_PASSWORD \
        --cni-version=$CNI_VERSION \
        --custom-image=$TARGET_IMAGE \
        --kubernetes-version=$KUBERNETES_VERSION
fi

./bin/delete-masterkube.sh

echo "Launch custom $MASTERKUBE instance with $TARGET_IMAGE"

hexchars="0123456789ABCDEF"
MACADDR1=$( for i in {1..6} ; do echo -n ${hexchars:$(( $RANDOM % 16 )):1} ; done | sed -e 's/\(..\)/:\1/g' )
MACADDR2=$( for i in {1..6} ; do echo -n ${hexchars:$(( $RANDOM % 16 )):1} ; done | sed -e 's/\(..\)/:\1/g' )
ADDR1=$(echo "00:16:3E:${MACADDR1}")
ADDR2=$(echo "00:16:3E:${MACADDR2}")


cat > ./config/network.yaml <<EOF
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
    eth1:
      gateway4: 10.65.4.1
      addresses:
      - 10.65.4.200/24
EOF

echo "$KUBERNETES_PASSWORD" > ./config/kubernetes-password.txt

cat > ./config/vendordata.yaml <<EOF
#cloud-config
package_update: true
package_upgrade: true
timezone: $TZ
ssh_authorized_keys:
    - $SSH_KEY
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
    "instance-id": "$(uuidgen)"
}
EOF

echo "#cloud-config" > ./config/userdata.yaml

cat <<EOF | tee ./config/userdata.json | python2 -c "import json,sys,yaml; print yaml.safe_dump(json.load(sys.stdin), width=500, indent=4, default_flow_style=False)" >> ./config/userdata.yaml
{
    "runcmd": [
        "echo 'Create $MASTERKUBE' > /var/log/masterkube.log"
    ]
}
EOF

gzip -c9 <./config/metadata.json | $BASE64 | tee > config/metadata.base64
gzip -c9 <./config/userdata.yaml | $BASE64 | tee > config/userdata.base64
gzip -c9 <./config/vendordata.yaml | $BASE64 | tee > config/vendordata.base64

echo "Clone $TARGET_IMAGE to $MASTERKUBE"

echo "TARGET_IMAGE=$TARGET_IMAGE"
echo "MASTERKUBE=$MASTERKUBE"

#### FOLDER=$(govc folder.info AFP | sed "s/\s\+//g" |  grep "Path\|Types" | sed "s/\nTypes/\|Types/g")

# Due to my vsphere center the folder name refer more path, so I need to precise the path instead
FOLDER_OPTIONS=
if [ "$GOVC_FOLDER" ]; then
    if [ ! $(govc folder.info $GOVC_FOLDER|grep Path|wc -l) -eq 1 ]; then
        FOLDER_OPTIONS="-folder=/$GOVC_DATACENTER/vm/$GOVC_FOLDER"
    fi
fi

govc vm.clone -link=false -on=false $FOLDER_OPTIONS -c=2 -m=4096 -vm=$TARGET_IMAGE $MASTERKUBE

echo "Set cloud-init settings for $MASTERKUBE"

govc vm.change -vm "${MASTERKUBE}" \
    -e guestinfo.metadata="$(cat config/metadata.base64)" \
    -e guestinfo.metadata.encoding="gzip+base64" \
    -e guestinfo.userdata="$(cat config/userdata.base64)" \
    -e guestinfo.userdata.encoding="gzip+base64" \
    -e guestinfo.vendordata="$(cat config/vendordata.base64)" \
    -e guestinfo.vendordata.encoding="gzip+base64" \

echo "Power On $MASTERKUBE"
govc vm.power -on "${MASTERKUBE}"

echo "Wait for IP from $MASTERKUBE"
IPADDR=$(govc vm.ip -wait 5m "${MASTERKUBE}")

echo "Prepare ${MASTERKUBE} instance"
scp -r bin $USER@${IPADDR}:~

echo "Start kubernetes ${MASTERKUBE} instance master node"
ssh $USER@${IPADDR} sudo mv /home/ubuntu/bin/* /usr/local/bin 
ssh $USER@${IPADDR} sudo create-cluster.sh flannel eth0 "$KUBERNETES_VERSION" "\'$PROVIDERID\'" 

scp $USER@${IPADDR}:/etc/cluster/* ./cluster

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

kubectl annotate node ${MASTERKUBE} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "cluster.autoscaler.nodegroup/node-index=0" "cluster.autoscaler.nodegroup/autoprovision=false" "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" --overwrite --kubeconfig=./cluster/config
kubectl label nodes ${MASTERKUBE} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "master=true" --overwrite --kubeconfig=./cluster/config
kubectl create secret tls kube-system -n kube-system --key ./etc/ssl/privkey.pem --cert ./etc/ssl/fullchain.pem --kubeconfig=./cluster/config

./bin/kubeconfig-merge.sh ${MASTERKUBE} cluster/config

echo "Write vmware cloud autoscaler provider config"

echo $(eval "cat <<EOF
$(<./templates/cluster/grpc-config.json)
EOF") | jq . >./config/grpc-config.json

if [ "$GOVC_INSECURE" == "1" ]; then
    INSECURE=true
else
    INSECURE=false
fi

cat <<EOF | jq . > config/kubernetes-vmware-autoscaler.json
{
    "network": "$TRANSPORT",
    "listen": "$LISTEN",
    "secret": "vsphere",
    "minNode": $MINNODES,
    "maxNode": $MAXNODES,
    "nodePrice": 0.0,
    "podPrice": 0.0,
    "image": "$TARGET_IMAGE",
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
        "package_upgrade": false
    },
    "sync-folder": {
    },
    "ssh-infos" : {
        "user": "kubernetes",
        "ssh-key": "$SSH_KEY"
    },
    "vsphere": {
        "default": {
            "url": "$GOVC_URL",
            "uid": "$GOVC_USERNAME",
            "password": "$GOVC_PASSWORD",
            "insecure": "$INSECURE",
            "dc" : "$GOVC_DATACENTER",
            "datastore": "$GOVC_DATASTORE",
            "resource-pool": "$GOVC_RESOURCE_POOL",
            "vmFolder": "$GOVC_FOLDER",
            "timeout": "300",
            "template-name": "$TARGET_IMAGE",
            "template": false,
            "linked": false,
            "customization": "$GOVC_CUSTOMIZATION",
            "network": {
                "dns": {
                    "search": [
                        "afp.com"
                    ],
                    "nameserver": [
                        "10.65.4.1"
                    ]
                },
                "interfaces": [
                    {
                        "exists": true,
                        "network": "AFP-MAINT-LYO",
                        "adapter": "vmxnet3",
                        "nic": "eth0",
                        "dhcp": true
                    },
                    {
                        "exists": true,
                        "network": "AFP-PROD-LYO",
                        "adapter": "vmxnet3",
                        "nic": "eth1",
                        "dhcp": true
                    }
                ]
            }
        },
        "${NODEGROUP_NAME}": {
            "url": "$GOVC_URL",
            "uid": "$GOVC_USERNAME",
            "password": "$GOVC_PASSWORD",
            "insecure": "$INSECURE",
            "dc" : "$GOVC_DATACENTER",
            "datastore": "$GOVC_DATASTORE",
            "resource-pool": "$GOVC_RESOURCE_POOL",
            "vmFolder": "$GOVC_FOLDER",
            "timeout": "300",
            "template-name": "$TARGET_IMAGE",
            "template": false,
            "linked": false,
            "customization": "$GOVC_CUSTOMIZATION",
            "network": {
                "dns": {
                    "search": [
                        "afp.com"
                    ],
                    "nameserver": [
                        "10.65.4.1"
                    ]
                },
                "interfaces": [
                    {
                        "exists": true,
                        "network": "AFP-MAINT-LYO",
                        "adapter": "vmxnet3",
                        "mac-address": "generate",
                        "nic": "eth0",
                        "dhcp": true
                    },
                    {
                        "exists": true,
                        "network": "AFP-PROD-LYO",
                        "adapter": "vmxnet3",
                        "mac-address": "generate",
                        "nic": "eth1",
                        "dhcp": true
                    }
                ]
            }
        }
    }
}
EOF

if [ "$OSDISTRO" == "Linux" ]; then
	sudo sed -i '/masterkube/d' /etc/hosts
else
	sudo sed -i '' '/masterkube/d' /etc/hosts
fi

sudo bash -c "echo '${IPADDR} ${MASTERKUBE}.${DOMAIN_NAME} masterkube.$DOMAIN_NAME masterkube-dashboard.$DOMAIN_NAME' >> /etc/hosts"

./bin/create-ingress-controller.sh
./bin/create-dashboard.sh
#./bin/create-autoscaler.sh
./bin/create-helloworld.sh

popd
