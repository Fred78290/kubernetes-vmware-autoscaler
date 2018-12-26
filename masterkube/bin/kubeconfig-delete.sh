#!/bin/sh

kubectl config delete-context k8s-$1-admin@k8s-$1
kubectl config delete-cluster k8s-$1
