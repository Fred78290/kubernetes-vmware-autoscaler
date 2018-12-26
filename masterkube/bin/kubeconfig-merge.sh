#!/bin/sh
cat $2 | sed -e "s/kubernetes/k8s-$1/g" > /tmp/k8s-$1.config

mkdir -p ~/.kube

cp ~/.kube/config ~/.kube/config.old

export KUBECONFIG=/tmp/k8s-$1.config:~/.kube/config.old

kubectl config view --flatten > ~/.kube/config

#rm /tmp/k8s-$1.config
