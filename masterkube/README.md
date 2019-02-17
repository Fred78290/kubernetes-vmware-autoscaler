# Introduction

This directory contains everthing to create an autoscaled cluster with vSphere.

## Prerequistes

Ensure that you have sudo right

You must also install

Linux Plateform
    govc
    libvirt
    python
    python-yaml

Darwin Plateform
    govc
    python
    python-yaml
    gnu-getopt

## Create the masterkube

The simply way to create the masterkube is to run [create-masterkube.sh](create-masterkube.sh)

Some needed file are located in:

| Name | Description |
| --- | --- |
| `bin` | Essentials scripts to build the master kubernetes node  |
| `etc/ssl`  | Your CERT for https. Autosigned will be generated if empty  |
| `template`  | Templates files to deploy pod & service |

The first thing done by this script is to create a VM Template Ubuntu-18.04.1 image with kubernetes and docker installed. The VM template will be named by default afp-slyo-bionic-kubernetes-(kuberneres version)

Next step will be to launch a cloned VM and create a master node. It will also deploy a dashboard at the URL https://masterkube-dashboard.@your-domain@/

To connect to the dashboard, copy paste the token from file [cluster/dashboard-token](./cluster/dashboard-token)

Next step is to deploy a replicaset helloworld. This replicaset use hostnetwork:true to enforce one pod per node.

During the process the script will create many files located in

| Name | Description |
| --- | --- |
| `cluster` | Essentials file to connect to kubernetes with kubeadm join  |
| `config`  | Configuration file generated during the build process  |

## Command line arguments

| Parameter | Description | Default |
| --- | --- |--- |
| `-c\|--no-custom-image` | Use standard image  | NO |
| `-k\|--ssh-private-key`  |Alternate ssh key file |~/.ssh/id_rsa|
| `-n\|--cni-version`  |CNI version |0.71
| `-p\|--password`  |Define the kubernetes user password |randomized|
| `-v\|--kubernetes-version`  |Which version of kubernetes to use |latest|
| `--max-nodes-total` | Maximum number of nodes in all node groups. Cluster autoscaler will not grow the cluster beyond this number. | 5 |
| `--cores-total` | Minimum and maximum number of cores in cluster, in the format < min >:< max >. Cluster autoscaler will not scale the cluster beyond these numbers. | 0:16 |
| `--memory-total` | Minimum and maximum number of gigabytes of memory in cluster, in the format < min >:< max >. Cluster autoscaler will not scale the cluster beyond these numbers. | 0:24 |
| `--max-autoprovisioned-node-group-count` | The maximum number of autoprovisioned groups in the cluster | 1 |
| `--scale-down-enabled` | Should CA scale down the cluster | true |
| `--scale-down-delay-after-add` | How long after scale up that scale down evaluation resumes | 1 minutes |
| `--scale-down-delay-after-delete` | How long after node deletion that scale down evaluation resumes, defaults to scan-interval | 1 minutes |
| `--scale-down-delay-after-failure` | How long after scale down failure that scale down evaluation resumes | 1 minutes |
| `--scale-down-unneeded-time` | How long a node should be unneeded before it is eligible for scale down | 1 minutes |
| `--scale-down-unready-time` | How long an unready node should be unneeded before it is eligible for scale down | 1 minutes |
| `--unremovable-node-recheck-timeout` | The timeout before we check again a node that couldn't be removed before | 1 minutes |
| `--net-address` | The public IP address | 10.0.0.200 |
| `--net-gateway` | The public IP gateway | 10.0.0.1 |
| `--net-dns` | The public IP dns | 10.0.0.1 |
| `--net-domain` | The public domain name | example.com |
| `--vm-private-network` | The name of private vSphere network | 'VM Network' |
| `--vm-public-network` | The name of private vSphere network | 'VM Public' |
| `--target-image` | The VM name created for cloning with kubernetes | bionic-kubernetes |
| `--seed-image` | The VM name used to created the targer image | bionic-server-cloudimg-seed |
| `--seed-user` | The cloud-init user name | ubuntu |

```bash
create-masterkube \
    --nodegroup=<My Group Name> \
    --target-image=<My VM template Name> \
    --seed-image=<My custom VM Template> \
    --seed-user=<My custom user> \
    --vm-private-network=<My private network> \
    --vm-public-network=<My public network> \
    --net-address="10.0.4.200" \
    --net-gateway="10.0.4.1" \
    --net-dns="10.0.4.1" \
    --net-domain="acme.com"
```

## Raise autoscaling

To scale up or down the cluster, just play with `kubectl scale`

To scale fresh masterkube `kubectl scale --replicas=2 deploy/helloworld -n kube-public`

## Delete master kube and worker nodes

To delete the master kube and associated worker nodes, just run the command [delete-masterkube.sh](./bin/delete-masterkube.sh)