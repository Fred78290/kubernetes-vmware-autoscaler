#/bin/bash

# This script create every thing to deploy a simple kubernetes autoscaled cluster with vmware.
# It will generate:
# Custom vmware image with every thing for kubernetes
# Config file to deploy the cluster autoscaler.
# kubectl run busybox --rm -ti --image=busybox -n kube-public /bin/sh

set -e

CURDIR=$(dirname $0)

export SCHEME="vmware"
export NODEGROUP_NAME="vmware-ca-k8s"
export MASTERKUBE="${NODEGROUP_NAME}-masterkube"
export PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
export SSH_PRIVATE_KEY="$HOME/.ssh/id_rsa"
export SSH_PUBLIC_KEY="${SSH_PRIVATE_KEY}.pub"
export KUBERNETES_VERSION=v1.22.1
export KUBERNETES_USER=kubernetes
export KUBERNETES_PASSWORD=
export KUBECONFIG=$HOME/.kube/config
export SEED_USER=ubuntu
export SEED_IMAGE="focal-server-cloudimg-seed"
export ROOT_IMG_NAME=focal-kubernetes
export TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
export CNI_VERSION="v0.9.1"
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
export NET_DOMAIN=home
export NET_IP=192.168.1.20
export NET_GATEWAY=10.0.0.1
export NET_DNS=10.0.0.1
export NET_MASK=255.255.255.0
export NET_MASK_CIDR=24
export VC_NETWORK_PRIVATE="Private Network"
export VC_NETWORK_PUBLIC="Public Network"
export METALLB_IP_RANGE=10.0.0.100-10.0.0.127
export REGISTRY=fred78290
export LAUNCH_CA=YES

#export GOVC_DATACENTER=
#export GOVC_DATASTORE=
#export GOVC_FOLDER=
#export GOVC_HOST=
#export GOVC_INSECURE=
#export GOVC_NETWORK=
#export GOVC_USERNAME=
#export GOVC_PASSWORD=
#export GOVC_RESOURCE_POOL=
#export GOVC_URL=
#export GOVC_VIM_VERSION="6.0"

SSH_OPTIONS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

# import govc hidden definitions
source ${CURDIR}/govc.defs

if [ "$OSDISTRO" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    alias base64="base64 -w 0"
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    alias sed=gsed
fi

TEMP=$(getopt -o k:n:p:s:t: --long node-group:,target-image:,seed-image:,seed-user:,vm-public-network:,vm-private-network:,net-address:,net-gateway:,net-dns:,net-domain:,transport:,ssh-private-key:,cni-version:,password:,kubernetes-version:,max-nodes-total:,cores-total:,memory-total:,max-autoprovisioned-node-group-count:,scale-down-enabled:,scale-down-delay-after-add:,scale-down-delay-after-delete:,scale-down-delay-after-failure:,scale-down-unneeded-time:,scale-down-unready-time:,unremovable-node-recheck-timeout: -n "$0" -- "$@")

eval set -- "$TEMP"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    --node-group)
        NODEGROUP_NAME="$2"
        MASTERKUBE="${NODEGROUP_NAME}-masterkube"
        PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
        shift 2
        ;;

    --target-image)
        ROOT_IMG_NAME="$2"
        TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
        shift 2
        ;;

    --seed-image)
        SEED_IMAGE="$2"
        shift 2
        ;;

    --seed-user)
        SEED_USER="$2"
        shift 2
        ;;

    --vm-private-network)
        VC_NETWORK_PRIVATE="$2"
        shift 2
        ;;

    --vm-public-network)
        VC_NETWORK_PUBLIC="$2"
        shift 2
        ;;

    --net-address)
        NET_IP="$2"
        shift 2
        ;;

    --net-gateway)
        NET_GATEWAY="$2"
        shift 2
        ;;

    --net-dns)
        NET_DNS="$2"
        shift 2
        ;;

    --net-domain)
        NET_DOMAIN="$2"
        shift 2
        ;;

    -d | --default-machine)
        DEFAULT_MACHINE="$2"
        shift 2
        ;;
    -s | --ssh-private-key)
        SSH_PRIVATE_KEY=$2
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

        # Same argument as cluster-autoscaler
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

if [ -z $KUBERNETES_PASSWORD ]; then
    if [ -f ~/.kubernetes_pwd ]; then
        KUBERNETES_PASSWORD=$(cat ~/.kubernetes_pwd)
    else
        KUBERNETES_PASSWORD=$(uuidgen)
        echo $n "$KUBERNETES_PASSWORD" > ~/.kubernetes_pwd
    fi
fi

export SSH_KEY_FNAME="$(basename $SSH_PRIVATE_KEY)"
export SSH_PUBLIC_KEY="${SSH_PRIVATE_KEY}.pub"
export SSH_KEY=$(cat "${SSH_PUBLIC_KEY}")

# GRPC network endpoint
if [ "$LAUNCH_CA" != "YES" ]; then
    SSH_PRIVATE_KEY_LOCAL="$SSH_PRIVATE_KEY"

    if [ "${TRANSPORT}" == "unix" ]; then
        LISTEN="/var/run/cluster-autoscaler/vmware.sock"
        CONNECTTO="unix:/var/run/cluster-autoscaler/vmware.sock"
    elif [ "${TRANSPORT}" == "tcp" ]; then
        if [ "${OSDISTRO}" == "Linux" ]; then
            NET_IF=$(ip route get 1 | awk '{print $5;exit}')
            IPADDR=$(ip addr show ${NET_IF} | grep -m 1 "inet\s" | tr '/' ' ' | awk '{print $2}')
        else
            NET_IF=$(route get 1 | grep -m 1 interface | awk '{print $2}')
            IPADDR=$(ifconfig ${NET_IF} | grep -m 1 "inet\s" | sed -n 1p | awk '{print $2}')
        fi

        LISTEN="${IPADDR}:5200"
        CONNECTTO="${IPADDR}:5200"
    else
        echo "Unknown transport: ${TRANSPORT}, should be unix or tcp"
        exit -1
    fi
else
    SSH_PRIVATE_KEY_LOCAL="/root/.ssh/id_rsa"
    TRANSPORT=unix
    LISTEN="/var/run/cluster-autoscaler/vmware.sock"
    CONNECTTO="unix:/var/run/cluster-autoscaler/vmware.sock"
fi

echo "Transport set to:${TRANSPORT}, listen endpoint at ${LISTEN}"

# Sample machine definition
MACHINE_DEFS=$(
    cat <<EOF
{
    "tiny": {
        "memsize": 1024,
        "vcpus": 1,
        "disksize": 10240
    },
    "small": {
        "memsize": 2048,
        "vcpus": 2,
        "disksize": 10240
    },
    "medium": {
        "memsize": 4096,
        "vcpus": 2,
        "disksize": 20480
    },
    "large": {
        "memsize": 8192,
        "vcpus": 4,
        "disksize": 51200
    },
    "extra-large": {
        "memsize": 16384,
        "vcpus": 4,
        "disksize": 102400
    }
}
EOF
)

pushd ${CURDIR}/../

[ -d config ] || mkdir -p config
[ -d cluster ] || mkdir -p cluster

export PATH=./bin:$PATH

# If CERT doesn't exist, create one autosigned
if [ ! -f ./etc/ssl/privkey.pem ]; then
    mkdir -p ./etc/ssl/
    openssl genrsa 2048 >./etc/ssl/privkey.pem
    openssl req -new -x509 -nodes -sha1 -days 3650 -key ./etc/ssl/privkey.pem >./etc/ssl/cert.pem
    cat ./etc/ssl/cert.pem ./etc/ssl/privkey.pem >./etc/ssl/fullchain.pem
    chmod 644 ./etc/ssl/*
fi

# Extract the domain name from CERT
export DOMAIN_NAME=$(openssl x509 -noout -subject -in ./etc/ssl/cert.pem | awk -F= '{print $NF}' | sed -e 's/^[ \t]*//' | sed 's/\*\.//g')

# If the VM template doesn't exists, build it from scrash
if [ -z "$(govc vm.info ${TARGET_IMAGE} 2>&1)" ]; then
    echo "Create vmware preconfigured image ${TARGET_IMAGE}"

    ./bin/create-image.sh \
        --password="${KUBERNETES_PASSWORD}" \
        --cni-version="${CNI_VERSION}" \
        --custom-image="${TARGET_IMAGE}" \
        --kubernetes-version="${KUBERNETES_VERSION}" \
        --seed="${SEED_IMAGE}" \
        --user="${SEED_USER}" \
        --ssh-key="${SSH_KEY}" \
        --primary-network="${VC_NETWORK_PUBLIC}" \
        --second-network="${VC_NETWORK_PRIVATE}"
fi

# Delete previous exixting version
delete-masterkube.sh

echo "Launch custom ${MASTERKUBE} instance with ${TARGET_IMAGE}"

cat > ./config/buildenv <<EOF
export SCHEME="$SCHEME"
export NODEGROUP_NAME="$NODEGROUP_NAME"
export MASTERKUBE="$MASTERKUBE"
export PROVIDERID="$PROVIDERID"
export SSH_PRIVATE_KEY=$SSH_PRIVATE_KEY
export SSH_PUBLIC_KEY=$SSH_PUBLIC_KEY
export SSH_KEY="$SSH_KEY"
export SSH_KEY_FNAME=$SSH_KEY_FNAME
export KUBERNETES_VERSION=$KUBERNETES_VERSION
export KUBERNETES_USER=${KUBERNETES_USER}
export KUBERNETES_PASSWORD=$KUBERNETES_PASSWORD
export KUBECONFIG=$KUBECONFIG
export SEED_USER=$SEED_USER
export SEED_IMAGE="$SEED_IMAGE"
export ROOT_IMG_NAME=$ROOT_IMG_NAME
export TARGET_IMAGE=$TARGET_IMAGE
export CNI_VERSION=$CNI_VERSION
export MINNODES=$MINNODES
export MAXNODES=$MAXNODES
export MAXTOTALNODES=$MAXTOTALNODES
export CORESTOTAL=$CORESTOTAL
export MEMORYTOTAL=$MEMORYTOTAL
export MAXAUTOPROVISIONNEDNODEGROUPCOUNT=$MAXAUTOPROVISIONNEDNODEGROUPCOUNT
export SCALEDOWNENABLED=$SCALEDOWNENABLED
export SCALEDOWNDELAYAFTERADD=$SCALEDOWNDELAYAFTERADD
export SCALEDOWNDELAYAFTERDELETE=$SCALEDOWNDELAYAFTERDELETE
export SCALEDOWNDELAYAFTERFAILURE=$SCALEDOWNDELAYAFTERFAILURE
export SCALEDOWNUNEEDEDTIME=$SCALEDOWNUNEEDEDTIME
export SCALEDOWNUNREADYTIME=$SCALEDOWNUNREADYTIME
export DEFAULT_MACHINE=$DEFAULT_MACHINE
export UNREMOVABLENODERECHECKTIMEOUT=$UNREMOVABLENODERECHECKTIMEOUT
export OSDISTRO=$OSDISTRO
export TRANSPORT=$TRANSPORT
export NET_DOMAIN=$NET_DOMAIN
export NET_IP=$NET_IP
export NET_GATEWAY=$NET_GATEWAY
export NET_DNS=$NET_DNS
export NET_MASK=$NET_MASK
export NET_MASK_CIDR=$NET_MASK_CIDR
export VC_NETWORK_PRIVATE=$VC_NETWORK_PRIVATE
export VC_NETWORK_PUBLIC=$VC_NETWORK_PUBLIC
export REGISTRY=$REGISTRY
export LAUNCH_CA=$LAUNCH_CA
EOF


cat >./config/network.yaml <<EOF
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
    eth1:
      gateway4: $NET_GATEWAY
      addresses:
      - $NET_IP/$NET_MASK_CIDR
EOF

echo "${KUBERNETES_PASSWORD}" >./config/kubernetes-password.txt

# Cloud init vendor-data
cat >./config/vendordata.yaml <<EOF
#cloud-config
package_update: true
package_upgrade: true
timezone: ${TZ}
ssh_authorized_keys:
    - ${SSH_KEY}
users:
    - default
system_info:
    default_user:
        name: ${KUBERNETES_USER}
EOF

# Cloud init meta-data
cat >./config/metadata.json <<EOF
{
    "network": "$(cat ./config/network.yaml | gzip -c9 | base64)",
    "network.encoding": "gzip+base64",
    "local-hostname": "${MASTERKUBE}",
    "instance-id": "$(uuidgen)"
}
EOF

# Cloud init user-data
cat > ./config/userdata.yaml <<EOF
#cloud-config
runcmd:
- echo "Create ${MASTERKUBE}" > /var/log/masterkube.log
EOF


gzip -c9 <./config/metadata.json | base64 | tee >config/metadata.base64
gzip -c9 <./config/userdata.yaml | base64 | tee >config/userdata.base64
gzip -c9 <./config/vendordata.yaml | base64 | tee >config/vendordata.base64

echo "Clone ${TARGET_IMAGE} to ${MASTERKUBE}"

echo "TARGET_IMAGE=${TARGET_IMAGE}"
echo "MASTERKUBE=${MASTERKUBE}"

# Due to my vsphere center the folder name refer more path, so I need to precise the path instead
FOLDER_OPTIONS=
if [ "${GOVC_FOLDER}" ]; then
    if [ ! $(govc folder.info ${GOVC_FOLDER} | grep -m 1 Path | wc -l) -eq 1 ]; then
        FOLDER_OPTIONS="-folder=/${GOVC_DATACENTER}/vm/${GOVC_FOLDER}"
    fi
fi

# Clone my template
govc vm.clone -link=false -on=false ${FOLDER_OPTIONS} -c=2 -m=4096 -vm=${TARGET_IMAGE} ${MASTERKUBE}

echo "Set cloud-init settings for ${MASTERKUBE}"

# Inject cloud-init elements
govc vm.change -vm "${MASTERKUBE}" \
    -e guestinfo.metadata="$(cat config/metadata.base64)" \
    -e guestinfo.metadata.encoding="gzip+base64" \
    -e guestinfo.userdata="$(cat config/userdata.base64)" \
    -e guestinfo.userdata.encoding="gzip+base64" \
    -e guestinfo.vendordata="$(cat config/vendordata.base64)" \
    -e guestinfo.vendordata.encoding="gzip+base64"

echo "Power On ${MASTERKUBE}"
govc vm.power -on "${MASTERKUBE}"

echo "Wait for IP from ${MASTERKUBE}"
IPADDR=$(govc vm.ip -wait 5m "${MASTERKUBE}")
VMHOST=$(govc vm.info "${MASTERKUBE}" | grep 'Host:' | awk '{print $2}')

echo "Prepare ${MASTERKUBE} instance"
govc host.autostart.add -host="${VMHOST}" "${MASTERKUBE}"
scp ${SSH_OPTIONS} -r bin ${KUBERNETES_USER}@${IPADDR}:~

echo "Start kubernetes ${MASTERKUBE} instance master node, kubernetes version=${KUBERNETES_VERSION}, providerID=${PROVIDERID}"
ssh ${SSH_OPTIONS} ${KUBERNETES_USER}@${IPADDR} sudo mv /home/${KUBERNETES_USER}/bin/* /usr/local/bin
ssh ${SSH_OPTIONS} ${KUBERNETES_USER}@${IPADDR} sudo create-cluster.sh --cni=flannel --net-if=eth1 --kubernetes-version="${KUBERNETES_VERSION}" --provider-id="'${PROVIDERID}'" --cert-extra-sans="${MASTERKUBE}.${DOMAIN_NAME},masterkube-vmware.${DOMAIN_NAME},masterkube-vmware-dashboard.${DOMAIN_NAME}"

echo "create cluster done"

scp -r ${SSH_OPTIONS} ${KUBERNETES_USER}@${IPADDR}:/etc/cluster/* ./cluster

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

kubectl annotate node ${MASTERKUBE} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "cluster.autoscaler.nodegroup/node-index=0" "cluster.autoscaler.nodegroup/autoprovision=false" "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" --overwrite --kubeconfig=./cluster/config
kubectl label nodes ${MASTERKUBE} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "master=true" --overwrite --kubeconfig=./cluster/config
kubectl create secret tls kube-system -n kube-system --key ./etc/ssl/privkey.pem --cert ./etc/ssl/fullchain.pem --kubeconfig=./cluster/config
kubectl create secret generic autoscaler-ssh-keys -n kube-system --from-file=id_rsa="${SSH_PRIVATE_KEY}" --from-file=id_rsa.pub="${SSH_PUBLIC_KEY}" --kubeconfig=./cluster/config

kubeconfig-merge.sh ${MASTERKUBE} cluster/config

echo "Write vsphere autoscaler provider config"

echo $(eval "cat <<EOF
$(<./templates/cluster/grpc-config.json)
EOF") | jq . >./config/grpc-config.json

if [ "${GOVC_INSECURE}" == "1" ]; then
    INSECURE=true
else
    INSECURE=false
fi

AUTOSCALER_CONFIG=$(cat <<EOF
{
    "network": "${TRANSPORT}",
    "listen": "${LISTEN}",
    "secret": "${SCHEME}",
    "minNode": ${MINNODES},
    "maxNode": ${MAXNODES},
    "nodePrice": 0.0,
    "podPrice": 0.0,
    "image": "${TARGET_IMAGE}",
    "optionals": {
        "pricing": false,
        "getAvailableMachineTypes": false,
        "newNodeGroup": false,
        "templateNodeInfo": false,
        "createNodeGroup": false,
        "deleteNodeGroup": false
    },
    "kubeadm": {
        "address": "${MASTER_IP}",
        "token": "${TOKEN}",
        "ca": "sha256:${CACERT}",
        "extras-args": [
            "--ignore-preflight-errors=All"
        ]
    },
    "default-machine": "${DEFAULT_MACHINE}",
    "machines": ${MACHINE_DEFS},
    "cloud-init": {
        "package_update": false,
        "package_upgrade": false
    },
    "ssh-infos" : {
        "user": "${KUBERNETES_USER}",
        "ssh-private-key": "${SSH_PRIVATE_KEY_LOCAL}"
    },
    "vmware": {
        "${NODEGROUP_NAME}": {
            "url": "${GOVC_URL}",
            "uid": "${GOVC_USERNAME}",
            "password": "${GOVC_PASSWORD}",
            "insecure": ${INSECURE},
            "dc" : "${GOVC_DATACENTER}",
            "datastore": "${GOVC_DATASTORE}",
            "resource-pool": "${GOVC_RESOURCE_POOL}",
            "vmFolder": "${GOVC_FOLDER}",
            "timeout": 300,
            "template-name": "${TARGET_IMAGE}",
            "template": false,
            "linked": false,
            "customization": "${GOVC_CUSTOMIZATION}",
            "network": {
                "dns": {
                    "search": [
                        "${NET_DOMAIN}"
                    ],
                    "nameserver": [
                        "${NET_DNS}"
                    ]
                },
                "interfaces": [
                    {
                        "exists": true,
                        "network": "${VC_NETWORK_PUBLIC}",
                        "adapter": "vmxnet3",
                        "mac-address": "generate",
                        "nic": "eth0",
                        "dhcp": true
                    },
                    {
                        "exists": true,
                        "network": "${VC_NETWORK_PRIVATE}",
                        "adapter": "vmxnet3",
                        "mac-address": "generate",
                        "nic": "eth1",
                        "dhcp": false,
                        "address": "${NET_IP}",
                        "gateway4": "${NET_GATEWAY}",
                        "netmask": "${NET_MASK}"
                    }
                ]
            }
        }
    }
}
EOF
)

echo "$AUTOSCALER_CONFIG" | jq . > config/kubernetes-vmware-autoscaler.json

# Recopy config file on master node
kubectl create configmap config-cluster-autoscaler --kubeconfig=./cluster/config -n kube-system \
	--from-file ./config/grpc-config.json \
	--from-file ./config/kubernetes-vmware-autoscaler.json

# Update /etc/hosts
sudo sed -i -E "/${MASTERKUBE}.${DOMAIN_NAME}/d" -E "/masterkube-vmware-dashboard.${DOMAIN_NAME}/d" /etc/hosts
sudo bash -c "echo '${IPADDR} ${MASTERKUBE}.${DOMAIN_NAME}' >> /etc/hosts"
sed -i -E "s/https:\/\/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:([0-9]+)/https:\/\/${MASTERKUBE}.${DOMAIN_NAME}:\1/g" cluster/config

# Create Pods
create-metallb.sh
create-ingress-controller.sh
create-dashboard.sh
create-metrics.sh
create-helloworld.sh
create-external-dns.sh

if [ "$LAUNCH_CA" != "NO" ]; then
    create-autoscaler.sh $LAUNCH_CA
fi

NGINX_IP=$(kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

sudo bash -c "echo '${NGINX_IP} masterkube-vmware.${DOMAIN_NAME} masterkube-vmware-dashboard.${DOMAIN_NAME}' >> /etc/hosts"

# Add cluster config in configmap
kubectl create configmap masterkube-config --kubeconfig=./cluster/config -n kube-system \
	--from-file ./cluster/ca.cert \
    --from-file ./cluster/dashboard-token \
    --from-file ./cluster/token

popd
