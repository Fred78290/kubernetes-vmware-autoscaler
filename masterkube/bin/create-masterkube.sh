#/bin/bash

# This script create every thing to deploy a simple kubernetes autoscaled cluster with AutoScaler.
# It will generate:
# Custom AutoScaler image with every thing for kubernetes
# Config file to deploy the cluster autoscaler.

CURDIR=$(dirname $0)

export CUSTOM_IMAGE=YES
export SSH_KEY=$(cat ~/.ssh/id_rsa.pub)
export KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
export KUBERNETES_PASSWORD=$(uuidgen)
export KUBECONFIG=$HOME/.kube/config
export TARGET_IMAGE=$HOME/.local/AutoScaler/cache/bionic-k8s-$KUBERNETES_VERSION-amd64.img
export CNI_VERSION="v0.7.1"
export PROVIDERID="AutoScaler://ca-grpc-AutoScaler/object?type=node&name=masterkube"
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
export TRANSPORT="unix"
export LOWBANDWIDTH="NO"

TEMP=$(getopt -o cli:k:n:p:t:v: --long low-bandwidth,transport:,no-custom-image,image:,ssh-key:,cni-version:,password:,kubernetes-version:,max-nodes-total:,cores-total:,memory-total:,max-autoprovisioned-node-group-count:,scale-down-enabled:,scale-down-delay-after-add:,scale-down-delay-after-delete:,scale-down-delay-after-failure:,scale-down-unneeded-time:,scale-down-unready-time:,unremovable-node-recheck-timeout: -n "$0" -- "$@")

eval set -- "$TEMP"

# extract options and their arguments into variables.
while true; do
	case "$1" in
	-c | --no-custom-image)
		CUSTOM_IMAGE="NO"
		shift 1
		;;
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
    -l | --low-bandwidth)
        LOWBANDWIDTH="YES"
        shift 1
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
		TARGET_IMAGE="$HOME/.local/AutoScaler/cache/bionic-k8s-$KUBERNETES_VERSION-amd64.img"
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
    LISTEN="/var/run/cluster-autoscaler/grpc.sock"
    CONNECTTO="/var/run/cluster-autoscaler/grpc.sock"
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

# Bandwidth. If low then fetch components before AutoScaler launch, because AutoScaler launch doesnt have timeout option control
if [ "$LOWBANDWIDTH" == "YES" ]; then
    echo "Low network low band width endorsed"

    KUBERNETES_CACHE="${HOME}/.local/kubernetes-vmware-autoscaler/${KUBERNETES_VERSION}"
    PACKAGE_UPGRADE="false"

    MOUNTPOINTS=$(cat <<EOF
        "$PWD/config": "/etc/cluster-autoscaler",
        "$KUBERNETES_CACHE/cni": "/opt/cni/bin",
        "$KUBERNETES_CACHE/kubernetes": "/opt/bin",
        "$KUBERNETES_CACHE/docker": "/opt/docker"
EOF
)

    if [ ! -d "$KUBERNETES_CACHE" ]; then
        mkdir -p "${HOME}/.local/kubernetes-vmware-autoscaler/${KUBERNETES_VERSION}"
        pushd $KUBERNETES_CACHE
        mkdir -p docker
        mkdir -p cni
        curl -L https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz | tar -C cni -xz
        mkdir -p kubernetes
        pushd kubernetes
        curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}
        chmod +x *
        popd
    fi

    RUN_CMD=$(
    cat <<EOF
    [
        "mkdir -p /opt/cni/bin",
        "mkdir -p /opt/bin",
        "curl https://get.docker.com | bash",
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
        "apt autoremove -y",
        "echo '#!/bin/bash' > /usr/local/bin/kubeimage",
        "echo 'for f in /opt/docker/*.tar ; do echo \"Import docker image cache \$f\" ; docker load -i \$f ; done' >> /usr/local/bin/kubeimage",
        "chmod +x /usr/local/bin/kubeimage"
]
EOF
)
else
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
fi

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
if [ "$OSDISTRO" == "Linux" ]; then
	POWERSTATE=$(
		cat <<EOF
        , "power_state": {
            "mode": "reboot",
            "message": "Reboot VM due upgrade",
            "condition": true
        }
EOF
	)
else
	CUSTOM_IMAGE="NO"
	POWERSTATE=
fi

if [ "$CUSTOM_IMAGE" == "YES" ] && [ ! -f $TARGET_IMAGE ]; then

	[ -d "$HOME/.local/AutoScaler/cache/" ] || mkdir -p $HOME/.local/AutoScaler/cache/

	if [ "$OSDISTRO" == "Linux" ]; then
		echo "Create AutoScaler preconfigured image"

		create-image.sh --password=$KUBERNETES_PASSWORD \
			--cni-version=$CNI_VERSION \
			--custom-image=$TARGET_IMAGE \
			--kubernetes-version=$KUBERNETES_VERSION
	else
		cat <<-EOF | python2 -c "import json,sys,yaml; print yaml.safe_dump(json.load(sys.stdin), width=500, indent=4, default_flow_style=False)" >./config/imagecreator.yaml
        {
            "package_update": true,
            "package_upgrade": false,
            "packages" : [
                "libguestfs-tools"
            ],
            "ssh_authorized_keys": [
                "$SSH_KEY"
            ]
        }
EOF
		echo "Create AutoScaler VM to create the custom image"

		AutoScaler launch -n imagecreator -m 4096 -c 4 --cloud-init=./config/imagecreator.yaml bionic

		ROOT_IMAGE=$(dirname $TARGET_IMAGE)

		AutoScaler mount $PWD/bin imagecreator:/masterkube/bin
		AutoScaler mount $HOME/.local/AutoScaler/cache/ imagecreator:/home/AutoScaler/.local/AutoScaler/cache/
		AutoScaler mount $ROOT_IMAGE imagecreator:$ROOT_IMAGE

		echo "Create AutoScaler preconfigured image (could take a long)"

		AutoScaler shell imagecreator <<EOF
            /masterkube/bin/create-image.sh --password=$KUBERNETES_PASSWORD \
                --cni-version=$CNI_VERSION \
                --custom-image=$TARGET_IMAGE \
                --kubernetes-version=$KUBERNETES_VERSION
            exit
EOF
		AutoScaler delete imagecreator -p
	fi
fi

./bin/delete-masterkube.sh

if [ "$CUSTOM_IMAGE" = "YES" ]; then
	echo "Launch custom masterkube instance with $TARGET_IMAGE"

	cat <<EOF | tee ./config/cloud-init-masterkube.json | python2 -c "import json,sys,yaml; print yaml.safe_dump(json.load(sys.stdin), width=500, indent=4, default_flow_style=False)" >./config/cloud-init-masterkube.yaml
    {
        "package_update": false,
        "package_upgrade": false,
        "users": $KUBERNETES_USER,
        "ssh_authorized_keys": [
            "$SSH_KEY"
        ],
        "group": [
            "kubernetes"
        ]
    }
EOF

	LAUNCH_IMAGE_URL=file://$TARGET_IMAGE

else
	echo "Launch standard masterkube instance"

	cat <<EOF | tee ./config/cloud-init-masterkube.json | python2 -c "import json,sys,yaml; print yaml.safe_dump(json.load(sys.stdin), width=500, indent=4, default_flow_style=False)" >./config/cloud-init-masterkube.yaml
    {
        "package_update": true,
        "package_upgrade": $PACKAGE_UPGRADE,
        "runcmd": $RUN_CMD,
        "users": $KUBERNETES_USER,
        "ssh_authorized_keys": [
            "$SSH_KEY"
        ],
        "group": [
            "kubernetes"
        ]
        $POWERSTATE
    }
EOF

	LAUNCH_IMAGE_URL="bionic"
fi

AutoScaler launch -n masterkube -m 4096 -c 2 -d 10G --cloud-init=./config/cloud-init-masterkube.yaml $LAUNCH_IMAGE_URL

# Due bug in AutoScaler MacOS, we need to reboot manually the VM after apt upgrade
if [ "$LOWBANDWIDTH" != "YES" ] && [ "$CUSTOM_IMAGE" != "YES" ] && [ "$OSDISTRO" != "Linux" ]; then
	AutoScaler stop masterkube
	AutoScaler start masterkube
fi

sudo mkdir -p /var/run/cluster-autoscaler

AutoScaler mount $PWD/bin masterkube:/masterkube/bin
AutoScaler mount $PWD/templates masterkube:/masterkube/templates
AutoScaler mount $PWD/etc masterkube:/masterkube/etc
AutoScaler mount $PWD/cluster masterkube:/etc/cluster
AutoScaler mount $PWD/kubernetes masterkube:/etc/kubernetes
AutoScaler mount $PWD/config masterkube:/etc/cluster-autoscaler
AutoScaler mount /var/run/cluster-autoscaler masterkube:/var/run/cluster-autoscaler

if [ "$LOWBANDWIDTH" == "YES" ]; then
    AutoScaler mount "$KUBERNETES_CACHE/kubernetes" masterkube:/opt/bin
    AutoScaler mount "$KUBERNETES_CACHE/docker" masterkube:/opt/docker
    AutoScaler mount "$KUBERNETES_CACHE/cni" masterkube:/opt/cni/bin
fi

echo "Prepare masterkube instance"

AutoScaler shell masterkube <<EOF
sudo usermod -aG docker AutoScaler
sudo usermod -aG docker kubernetes
echo "Start kubernetes masterkube instance master node"
sudo /usr/local/bin/kubeimage
sudo bash -c "export PATH=/opt/bin:/opt/cni/bin:/masterkube/bin:\$PATH; create-cluster.sh flannel ens3 '$KUBERNETES_VERSION' '$PROVIDERID'"
exit
EOF

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

if [ "$CUSTOM_IMAGE" = "YES" ]; then

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
else
	cat <<EOF | jq . > config/kubernetes-vmware-autoscaler.json
    {
        "network": "$TRANSPORT",
        "listen": "$LISTEN",
        "secret": "AutoScaler",
        "minNode": $MINNODES,
        "maxNode": $MAXNODES,
        "nodePrice": 0.0,
        "podPrice": 0.0,
        "image": "bionic",
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
            "package_update": true,
            "package_upgrade": $PACKAGE_UPGRADE,
            "runcmd": $RUN_CMD,
            "users": $KUBERNETES_USER,
            "ssh_authorized_keys": [
                "$SSH_KEY"
            ],
            "group": [
                "kubernetes"
            ]
            $POWERSTATE
        },
        "mount-point": {
            $MOUNTPOINTS
        }
    }
EOF

fi

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
