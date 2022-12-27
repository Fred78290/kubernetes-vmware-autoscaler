[![Build Status](https://github.com/fred78290/kubernetes-vmware-autoscaler/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/Fred78290/Fred78290_kubernetes-vmware-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-vmware-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-vmware-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-vmware-autoscaler/blob/master/LICENSE)

# kubernetes-vmware-autoscaler

Kubernetes autoscaler for vsphere/esxi including a custom resource controller to create managed node without code

### Supported releases ###

* 1.24.6
    - This version is supported kubernetes v1.24
* 1.25.5
    - This version is supported kubernetes v1.25
* 1.26.
    - This version is supported kubernetes v1.26

### Unmaintened releases

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

## How it works

This tool will drive vSphere to deploy VM at the demand. The cluster autoscaler deployment use my enhanced version of [cluster-autoscaler](https://github.com/Fred78290/autoscaler).

This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here [cluster-autoscaler](https://hub.docker.com/r/fred78290/cluster-autoscaler)

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/cluster-autoscaler.yaml). You must fill value between <>

### Before you must create a kubernetes cluster on vSphere

You can do it from scrash or you can use script from project [autoscaled-masterkube-vmware](https://github.com/Fred78290/autoscaled-masterkube-vmware) to create a kubernetes cluster in single control plane or in HA mode with 3 control planes.

## Commandline arguments

| Parameter | Description |
| --- | --- |
| `version` | Print the version and exit  |
| `save`  | Tell the tool to save state in this file  |
| `config`  |The the tool to use config file |

## Build

The build process use make file. The simplest way to build is `make container`

# New features

## Network

Now it's possible to disable dhcp-default routes

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
	"network": "unix",
	"listen": "/var/run/cluster-autoscaler/vmware.sock",
	"secret": "vmware",
	"minNode": 0,
	"maxNode": 9,
	"maxNode-per-cycle": 2,
	"node-name-prefix": "autoscaled",
	"managed-name-prefix": "managed",
	"controlplane-name-prefix": "master",
	"nodePrice": 0,
	"podPrice": 0,
	"image": "focal-kubernetes-cni-flannel-v1.25.2-containerd-amd64",
	"optionals": {
		"pricing": false,
		"getAvailableMachineTypes": false,
		"newNodeGroup": false,
		"templateNodeInfo": false,
		"createNodeGroup": false,
		"deleteNodeGroup": false
	},
	"kubeadm": {
		"address": "x.x.x.x:6443",
		"token": "....",
		"ca": "sha256:....",
		"extras-args": [
			"--ignore-preflight-errors=All"
		]
	},
	"default-machine": "large",
	"machines": {
		"tiny": {
			"memsize": 2048,
			"vcpus": 2,
			"disksize": 10240
		},
		"small": {
			"memsize": 4096,
			"vcpus": 2,
			"disksize": 20480
		},
		"medium": {
			"memsize": 4096,
			"vcpus": 4,
			"disksize": 20480
		},
		"large": {
			"memsize": 8192,
			"vcpus": 4,
			"disksize": 51200
		},
		"xlarge": {
			"memsize": 16384,
			"vcpus": 4,
			"disksize": 102400
		},
		"2xlarge": {
			"memsize": 16384,
			"vcpus": 8,
			"disksize": 102400
		},
		"4xlarge": {
			"memsize": 32768,
			"vcpus": 8,
			"disksize": 102400
		}
	},
	"cloud-init": {
		"package_update": false,
		"package_upgrade": false,
		"runcmd": [
			"echo 1 > /sys/block/sda/device/rescan",
			"growpart /dev/sda 1",
			"resize2fs /dev/sda1",
			"echo 'x.x.x.x vmware-ca-k8s-masterkube vmware-ca-k8s-masterkube.acme.com' >> /etc/hosts"
		]
	},
	"ssh-infos": {
		"user": "kubernetes",
		"ssh-private-key": "/root/.ssh/id_rsa"
	},
	"vmware": {
		"vmware-ca-k8s": {
			"url": "https://administrator@vsphere.acme.com:userpasswrd@vsphere.acme.com/sdk",
			"uid": "administrator@vsphere.acme.com",
			"password": "userpasswrd",
			"insecure": true,
			"dc": "DC01",
			"datastore": "datastore1",
			"resource-pool": "HOME/Resources/FR",
			"vmFolder": "HOME",
			"timeout": 300,
			"template-name": "focal-kubernetes-cni-flannel-v1.25.2-containerd-amd64",
			"template": false,
			"linked": false,
			"customization": "",
			"network": {
				"domain": "acme.com",
				"dns": {
					"search": [
						"acme.com"
					],
					"nameserver": [
						"Y.Y.Y.Y"
					]
				},
				"interfaces": [
					{
						"primary": false,
						"exists": true,
						"network": "VM Network",
						"adapter": "vmxnet3",
						"mac-address": "generate",
						"nic": "eth0",
						"dhcp": true,
						"use-dhcp-routes": true,
						"routes": [
							{
								"to": "A.B.C.D/16",
								"via": "10.0.0.253",
								"metric": 100
							},
							{
								"to": "E.F.G.H/8",
								"via": "10.0.0.253",
								"metric": 500
							}
						]
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
						"address": "192.168.1.26",
						"gateway": "10.0.0.1",
						"netmask": "255.255.255.0",
						"routes": []
					}
				]
			}
		}
	}
}
```
