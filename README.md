[![Build Status](https://github.com/fred78290/kubernetes-vmware-autoscaler/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Fred78290/Fred78290_kubernetes-vmware-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-vmware-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-vmware-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-vmware-autoscaler/blob/master/LICENSE)

# kubernetes-vmware-autoscaler

Kubernetes autoscaler for vsphere/esxi including a custom resource controller to create managed node without code

### Supported releases ###

* 1.21.0
    - This version is supported kubernetes v1.21
* 1.22.0
    - This version is supported kubernetes v1.22
* 1.23.0
    - This version is supported kubernetes v1.23

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

## How it works

This tool will drive vSphere to deploy VM at the demand. The cluster autoscaler deployment use my enhanced version of [cluster-autoscaler](https://github.com/Fred78290/autoscaler).

This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here [cluster-autoscaler](https://hub.docker.com/r/fred78290/cluster-autoscaler)

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/cluster-autoscaler.yaml). You must fill value between <>

### Before you must create a kubernetes cluster on vSphere

You can do it from scrash or you can use script from projetct [autoscaled-masterkube-vmware](https://github.com/Fred78290/autoscaled-masterkube-vmware) to create a kubernetes cluster in single control plane or in HA mode with 3 control planes.

## Commandline arguments

| Parameter | Description |
| --- | --- |
| `version` | Print the version and exit  |
| `save`  | Tell the tool to save state in this file  |
| `config`  |The the tool to use config file |

## Build

The build process use make file. The simplest way to build is `make container`

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
  - demo-label.aldunelabs.com=demo
  - sample-label.aldunelabs.com=sample
  annotations:
  - demo-annotation.aldunelabs.com=demo
  - sample-annotation.aldunelabs.com=sample
  networks:
    -
      network: "VM Network"
      address: 10.0.0.80
      netmask: 255.255.255.0
      gateway: 10.0.0.1
    -
      network: "VM Private"
      address: 192.168.1.80
      netmask: 255.255.255.0
      gateway: 10.0.0.1
```
