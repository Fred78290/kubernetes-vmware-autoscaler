[![Build Status](https://github.com/fred78290/kubernetes-vmware-autoscaler/actions/workflows/ci.yml/badge.svg?branch=v1.20.5)](https://github.com/Fred78290/Fred78290_kubernetes-vmware-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-vmware-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-vmware-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-vmware-autoscaler/blob/master/LICENSE)

# kubernetes-vmware-autoscaler

Kubernetes autoscaler for vsphere/esxi

### Supported releases ###

* 1.20.5
    - This version is supported kubernetes v1.20
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

## How it works

This tool will drive vSphere to deploy VM at the demand. The cluster autoscaler deployment use my enhanced version of [cluster-autoscaler](https://github.com/Fred78290/autoscaler).

This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here [cluster-autoscaler](https://hub.docker.com/r/fred78290/cluster-autoscaler)

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/cluster-autoscaler.yaml). You must fill value between <>

### Before you must create a kubernetes cluster on vSphere

You can do it from scrash or you can use script from projetct [autoscaled-masterkube-vmware](https://github.com/Fred78290/autoscaled-masterkube-vmware)  to create a kubernetes cluster in single control plane or in HA mode with 3 control planes.

## Commandline arguments

| Parameter | Description |
| --- | --- |
| `version` | Print the version and exit  |
| `save`  | Tell the tool to save state in this file  |
| `config`  |The the tool to use config file |

## Build

The build process use make file. The simplest way to build is `make container`
