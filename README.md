[![Build Status](https://github.com/fred78290/kubernetes-vmware-autoscaler/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/Fred78290/Fred78290_kubernetes-vmware-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-vmware-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-vmware-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-vmware-autoscaler/blob/master/LICENSE)

# kubernetes-vmware-autoscaler

Kubernetes autoscaler for vsphere/esxi including a custom resource controller to create managed node without code

### Supported releases ###

* 1.26.11
    - This version is supported kubernetes v1.26 and support k3s, rke2, external kubernetes distribution
* 1.27.8
    - This version is supported kubernetes v1.27 and support k3s, rke2, external kubernetes distribution
* 1.28.1
    - This version is supported kubernetes v1.28 and support k3s, rke2, external kubernetes distribution

## How it works

This tool will drive vSphere to deploy VM at the demand. The cluster autoscaler deployment use vanilla cluster-autoscaler or my enhanced version of [cluster-autoscaler](https://github.com/Fred78290/autoscaler).

This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here [cluster-autoscaler](https://hub.docker.com/r/fred78290/cluster-autoscaler)

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/kubeadm/cluster-autoscaler.yaml). You must fill value between <>

### Before you must create a kubernetes cluster on vSphere

You can do it from scrash or you can use script from project [autoscaled-masterkube-vmware](https://github.com/Fred78290/autoscaled-masterkube-vmware) to create a kubernetes cluster in single control plane or in HA mode with 3 control planes.

## Commandline arguments

| Parameter | Description |
| --- | --- |
| `version` | Display version and exit |
| `save` | Tell the tool to save state in this file |
| `config` |The the tool to use config file |
| `log-format` | The format in which log messages are printed (default: text, options: text, json)|
| `log-level` | Set the level of logging. (default: info, options: panic, debug, info, warning, error, fatal)|
| `debug` | Debug mode|
| `distribution` | Which kubernetes distribution to use: kubeadm, k3s, rke2, external|
| `use-vanilla-grpc` | Tell we use vanilla autoscaler externalgrpc cloudprovider|
| `use-controller-manager` | Tell we use vsphere controller manager|
| `use-external-etcd` | Tell we use an external etcd service (overriden by config file if defined)|
| `src-etcd-ssl-dir` | Locate the source etcd ssl files (overriden by config file if defined)|
| `dst-etcd-ssl-dir` | Locate the destination etcd ssl files (overriden by config file if defined)|
| `kubernetes-pki-srcdir` | Locate the source kubernetes pki files (overriden by config file if defined)|
| `kubernetes-pki-dstdir` | Locate the destination kubernetes pki files (overriden by config file if defined)|
| `server` | The Kubernetes API server to connect to (default: auto-detect)|
| `kubeconfig` | Retrieve target cluster configuration from a Kubernetes configuration file (default: auto-detect)|
| `request-timeout` | Request timeout when calling Kubernetes APIs. 0s means no timeout|
| `deletion-timeout` | Deletion timeout when delete node. 0s means no timeout|
| `node-ready-timeout` | Node ready timeout to wait for a node to be ready. 0s means no timeout|
| `max-grace-period` | Maximum time evicted pods will be given to terminate gracefully.|
| `min-cpus` | Limits: minimum cpu (default: 1)|
| `max-cpus` | Limits: max cpu (default: 24)|
| `min-memory` | Limits: minimum memory in MB (default: 1G)|
| `max-memory` | Limits: max memory in MB (default: 24G)|
| `min-managednode-cpus` | Managed node: minimum cpu (default: 2)|
| `max-managednode-cpus` | Managed node: max cpu (default: 32)|
| `min-managednode-memory` | Managed node: minimum memory in MB (default: 2G)|
| `max-managednode-memory` | Managed node: max memory in MB (default: 24G)|
| `min-managednode-disksize` | Managed node: minimum disk size in MB (default: 10MB)|
| `max-managednode-disksize` | Managed node: max disk size in MB (default: 1T)|

## Build

The build process use make file. The simplest way to build is `make container`

# New features

## Use k3s, rke2 or external as kubernetes distribution method

Instead using **kubeadm** as kubernetes distribution method, it is possible to use **k3s**, **rke2** or **external**

**external** allow to use custom shell script to join cluster

Samples provided here

* [kubeadm](./examples/kubeadm/cluster-autoscaler.yaml)
* [rke2](./examples/rke2/cluster-autoscaler.yaml)
* [k3s](./examples/rke2/cluster-autoscaler.yaml)
* [external](./examples/external/cluster-autoscaler.yaml)

## Use the vanilla autoscaler with extern gRPC cloud provider

You can also use the vanilla autoscaler with the [externalgrpc cloud provider](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/cloudprovider/externalgrpc)

Samples of the cluster-autoscaler deployment with vanilla autoscaler. You must fill value between <>

* [kubeadm](./examples/kubeadm/cluster-autoscaler-vanilla.yaml)
* [rke2](./examples/rke2/cluster-autoscaler-vanilla.yaml)
* [k3s](./examples/rke2/cluster-autoscaler-vanilla.yaml)
* [external](./examples/external/cluster-autoscaler-vanilla.yaml)

## Use external kubernetes distribution

When you use a custom method to create your cluster, you must provide a shell script to vmware-autoscaler to join the cluster. The script use a yaml config created by vmware-autscaler at the given path.

config: /etc/default/vmware-autoscaler-config.yaml

```
provider-id: vsphere://42373f8d-b72d-21c0-4299-a667a18c9fce
max-pods: 110
node-name: vmware-dev-rke2-woker-01
server: 192.168.1.120:9345
token: K1060b887525bbfa7472036caa8a3c36b550fbf05e6f8e3dbdd970739cbd7373537
disable-cloud-controller: false
````

If you declare to use an external etcd service

```
datastore-endpoint: https://1.2.3.4:2379
datastore-cafile: /etc/ssl/etcd/ca.pem
datastore-certfile: /etc/ssl/etcd/etcd.pem
datastore-keyfile: /etc/ssl/etcd/etcd-key.pem
```

You can also provide extras config onto this file

```
"external": {
  "join-command": "/usr/local/bin/join-cluster.sh"
  "config-path": "/etc/default/vmware-autoscaler-config.yaml"
  "extra-config": {
      "mydata": {
        "extra": "ball"
      },
      "...": "..."
  }
},
```

Your script is responsible to set the correct kubelet flags such as max-pods=110, provider-id=vsphere://42373f8d-b72d-21c0-4299-a667a18c9fce, cloud-provider=external, ...

## Annotations requirements

If you expected to use vmware-autoscaler on already deployed kubernetes cluster, you must add some node annotations to existing node

Also don't forget to create an image usable by vmware-autoscaler to scale up the cluster [create-image.sh](https://raw.githubusercontent.com/Fred78290/autoscaled-masterkube-vmware/master/bin/create-image.sh)

| Annotation | Description | Value |
| --- | --- | --- |
| `cluster-autoscaler.kubernetes.io/scale-down-disabled` | Avoid scale down for this node |true|
| `cluster.autoscaler.nodegroup/name` | Node group name |vmware-dev-rke2|
| `cluster.autoscaler.nodegroup/autoprovision` | Tell if the node is provisionned by vmware-autoscaler |false|
| `cluster.autoscaler.nodegroup/instance-id` | The vm UUID |42373f8d-b72d-21c0-4299-a667a18c9fce|
| `cluster.autoscaler.nodegroup/managed` | Tell if the node is managed by vmware-autoscaler not autoscaled |false|
| `cluster.autoscaler.nodegroup/node-index` | The node index, will be set if missing |0|

Sample master node

```
    cluster-autoscaler.kubernetes.io/scale-down-disabled: "true"
    cluster.autoscaler.nodegroup/autoprovision: "false"
    cluster.autoscaler.nodegroup/instance-id: 42373f8d-b72d-21c0-4299-a667a18c9fce
    cluster.autoscaler.nodegroup/managed: "false" 
    cluster.autoscaler.nodegroup/name: vmware-dev-rke2
    cluster.autoscaler.nodegroup/node-index: "0"
```

Sample first worker node

```
    cluster-autoscaler.kubernetes.io/scale-down-disabled: "true"
    cluster.autoscaler.nodegroup/autoprovision: "false"
    cluster.autoscaler.nodegroup/instance-id: 42370879-d4f7-eab0-a1c2-918a97ac6856
    cluster.autoscaler.nodegroup/managed: "false"
    cluster.autoscaler.nodegroup/name: vmware-dev-rke2
    cluster.autoscaler.nodegroup/node-index: "1"
```

Sample autoscaled worker node

```
    cluster-autoscaler.kubernetes.io/scale-down-disabled: "false"
    cluster.autoscaler.nodegroup/autoprovision: "true"
    cluster.autoscaler.nodegroup/instance-id: 3d25c629-3f1d-46b3-be9f-b95db2a64859
    cluster.autoscaler.nodegroup/managed: "false"
    cluster.autoscaler.nodegroup/name: vmware-dev-rke2
    cluster.autoscaler.nodegroup/node-index: "2"
```

## Node labels

These labels will be added

| Label | Description | Value |
| --- | --- | --- |
|`node-role.kubernetes.io/control-plane`|Tell if the node is control-plane |true|
|`node-role.kubernetes.io/master`|Tell if the node is master |true|
|`node-role.kubernetes.io/worker`|Tell if the node is worker |true|

## Network

Now it's possible to disable dhcp-default routes and custom route

## VMWare CPI compliant

Version 1.24.6 and 1.25.2 and above are [vsphere cloud provider](https://github.com/kubernetes/cloud-provider-vsphere) by building provider-id conform to syntax `vsphere://<VM UUID>`

## CRD controller

This new release include a CRD controller allowing to create kubernetes node without use of govc or code. Just by apply a configuration file, you have the ability to create nodes on the fly.

As exemple you can take a look on [artifacts/examples/example.yaml](artifacts/examples/example.yaml) on execute the following command to create a new node

```
kubectl apply -f artifacts/examples/example.yaml
```

If you want delete the node just delete the CRD with the call

```
kubectl delete -f artifacts/examples/example.yaml
```

You have the ability also to create a control plane as instead a worker

```
kubectl apply -f artifacts/examples/controlplane.yaml
```

The resource is cluster scope so you don't need a namespace. The name of the resource is not the name of the managed node.

The minimal resource declaration

```
apiVersion: "nodemanager.aldunelabs.com/v1alpha1"
kind: "ManagedNode"
metadata:
  name: "vmware-ca-k8s-managed-01"
spec:
  nodegroup: vmware-ca-k8s
  vcpus: 2
  memorySizeInMb: 2048
  diskSizeInMb: 10240
```

The full qualified resource including networks declaration to override the default controller network management and adding some node labels & annotations. If you specify the managed node as controller, you can also allows the controlplane to support deployment as a worker node

```
apiVersion: "nodemanager.aldunelabs.com/v1alpha1"
kind: "ManagedNode"
metadata:
  name: "vmware-ca-k8s-managed-01"
spec:
  nodegroup: vmware-ca-k8s
  controlPlane: false
  allowDeployment: false
  vcpus: 2
  memorySizeInMb: 2048
  diskSizeInMb: 10240
  labels:
  - demo-label.acme.com=demo
  - sample-label.acme.com=sample
  annotations:
  - demo-annotation.acme.com=demo
  - sample-annotation.acme.com=sample
  networks:
    -
      network: "VM Network"
      address: 10.0.0.80
      netmask: 255.255.255.0
      gateway: 10.0.0.1
      use-dhcp-routes: false
      routes:
        - to: x.x.0.0/16
          via: 10.0.0.253
          metric: 100
        - to: y.y.y.y/8
          via: 10.0.0.253
          metric: 500
    -
      network: "VM Private"
      address: 192.168.1.80
      netmask: 255.255.255.0
      use-dhcp-routes: false
```

# Declare additional routes and disable default DHCP routes

The release 1.24 and above allows to add additionnal route per interface, it also allows to disable default route declared by DHCP server.

As example of use generated by autoscaled-masterkube-vmware scripts

```
{
  "use-external-etcd": false,
  "src-etcd-ssl-dir": "/etc/etcd/ssl",
  "dst-etcd-ssl-dir": "/etc/kubernetes/pki/etcd",
  "kubernetes-pki-srcdir": "/etc/kubernetes/pki",
  "kubernetes-pki-dstdir": "/etc/kubernetes/pki",
  "distribution": "rke2",
  "network": "unix",
  "listen": "/var/run/cluster-autoscaler/vmware.sock",
  "cert-private-key": "/etc/ssl/client-cert/tls.key",
  "cert-public-key": "/etc/ssl/client-cert/tls.crt",
  "cert-ca": "/etc/ssl/client-cert/ca.crt",
  "secret": "vmware",
  "minNode": 0,
  "maxNode": 9,
  "maxNode-per-cycle": 2,
  "node-name-prefix": "autoscaled",
  "managed-name-prefix": "managed",
  "controlplane-name-prefix": "master",
  "nodePrice": 0,
  "podPrice": 0,
  "image": "jammy-kubernetes-cni-flannel-v1.27.8-containerd-amd64",
  "optionals":
    {
      "pricing": false,
      "getAvailableMachineTypes": false,
      "newNodeGroup": false,
      "templateNodeInfo": false,
      "createNodeGroup": false,
      "deleteNodeGroup": false,
    },
  "kubeadm":
    {
      "address": "192.168.1.120:6443",
      "token": "h1g55p.hm4rg52ymloax182",
      "ca": "sha256:c7a86a7a9a03a628b59207f4f3b3e038ebd03260f3ad5ba28f364d513b01f542",
      "extras-args": ["--ignore-preflight-errors=All"],
    },
  "k3s":
    {
      "datastore-endpoint": "",
      "extras-commands": []
    },
  "external":
    {
      "join-command": "/usr/local/bin/join-cluster.sh"
      "config-path": "/etc/default/vmware-autoscaler-config.yaml"
      "extra-config": {
        "...": "..."
      }
    },
  "default-machine": "large",
  "machines":
    {
      "tiny": { "memsize": 2048, "vcpus": 2, "disksize": 10240 },
      "small": { "memsize": 4096, "vcpus": 2, "disksize": 20480 },
      "medium": { "memsize": 4096, "vcpus": 4, "disksize": 20480 },
      "large": { "memsize": 8192, "vcpus": 4, "disksize": 51200 },
      "xlarge": { "memsize": 16384, "vcpus": 4, "disksize": 102400 },
      "2xlarge": { "memsize": 16384, "vcpus": 8, "disksize": 102400 },
      "4xlarge": { "memsize": 32768, "vcpus": 8, "disksize": 102400 },
    },
  "node-labels":
    [
      "topology.kubernetes.io/region=home",
      "topology.kubernetes.io/zone=office",
      "topology.csi.vmware.com/k8s-region=home",
      "topology.csi.vmware.com/k8s-zone=office",
    ],
  "cloud-init":
    {
      "package_update": false,
      "package_upgrade": false,
      "runcmd":
        [
          "echo 1 > /sys/block/sda/device/rescan",
          "growpart /dev/sda 1",
          "resize2fs /dev/sda1",
          "echo '192.168.1.120 vmware-ca-k8s-masterkube vmware-ca-k8s-masterkube.acme.com' >> /etc/hosts",
        ],
    },
  "ssh-infos": { "user": "kubernetes", "ssh-private-key": "/root/.ssh/id_rsa" },
  "autoscaling-options":
    {
      "scaleDownUtilizationThreshold": 0.5,
      "scaleDownGpuUtilizationThreshold": 0.5,
      "scaleDownUnneededTime": "1m",
      "scaleDownUnreadyTime": "1m",
    },
  "vmware":
    {
      "vmware-ca-k8s":
        {
          "url": "https://administrator@acme.com:mySecret@vsphere.acme.com/sdk",
          "uid": "administrator@vsphere.acme.com",
          "password": "mySecret",
          "insecure": true,
          "dc": "DC01",
          "datastore": "datastore1",
          "resource-pool": "ACME/Resources/FR",
          "vmFolder": "HOME",
          "timeout": 300,
          "template-name": "jammy-kubernetes-cni-flannel-v1.26.0-containerd-amd64",
          "template": false,
          "linked": false,
          "customization": "",
          "network":
            {
              "domain": "acme.com",
              "dns": { "search": ["acme.com"], "nameserver": ["10.0.0.1"] },
              "interfaces":
                [
                  {
                    "primary": false,
                    "exists": true,
                    "network": "VM Network",
                    "adapter": "vmxnet3",
                    "mac-address": "generate",
                    "nic": "eth0",
                    "dhcp": true,
                    "use-dhcp-routes": true,
                    "routes":
                      [
                        {
                          "to": "172.30.0.0/16",
                          "via": "10.0.0.5",
                          "metric": 500,
                        },
                      ],
                  },
                  {
                    "primary": true,
                    "exists": true,
                    "network": "VM Private",
                    "adapter": "vmxnet3",
                    "mac-address": "generate",
                    "nic": "eth1",
                    "dhcp": true,
                    "use-dhcp-routes": false,
                    "address": "192.168.1.124",
                    "gateway": "10.0.0.1",
                    "netmask": "255.255.255.0",
                    "routes": [],
                  },
                ],
            },
        },
    },
}

```

# Unmaintened releases

* 1.15.11
    - This version is supported kubernetes v1.15
* 1.16.9
    - This version is supported kubernetes v1.16
* 1.17.5
    - This version is supported kubernetes v1.17
* 1.18.2
    - This version is supported kubernetes v1.18
* 1.19.0
    - This version is supported kubernetes v1.19
* 1.20.5
    - This version is supported kubernetes v1.20
* 1.20.5
    - This version is supported kubernetes v1.20
* 1.21.0
    - This version is supported kubernetes v1.21
* 1.22.0
    - This version is supported kubernetes v1.22
* 1.23.0
    - This version is supported kubernetes v1.23
* 1.24.0
    - This version is supported kubernetes v1.24
* 1.24.2
    - This version is supported kubernetes v1.24
* 1.24.3
    - This version is supported kubernetes v1.24
* 1.24.6
    - This version is supported kubernetes v1.24
* 1.25.6
    - This version is supported kubernetes v1.25
* 1.25.7
    - This version is supported kubernetes v1.25 and support k3s
* 1.26.1
    - This version is supported kubernetes v1.26
