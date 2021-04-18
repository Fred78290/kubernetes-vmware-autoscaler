#!/bin/bash
VERSION=v1.21.0
REGISTRY=devregistry.aldunelabs.com

make -e REGISTRY=$REGISTRY -e TAG=$VERSION container

docker push $REGISTRY/vsphere-autoscaler:$VERSION
