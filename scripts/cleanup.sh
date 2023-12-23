#!/bin/bash
SRCDIR=$(dirname $0)
CONFIG=$1
CONFIG_DIR=${SRCDIR}/../.config/$1

rm -rf /tmp/vmware.sock
sudo mkdir -p /var/run/cluster-autoscaler/
sudo chown $USER /var/run/cluster-autoscaler/
rm -f /var/run/cluster-autoscaler/vmware.sock

AUTOSCALER_VMWARE=${HOME}/Projects/GitHub/autoscaled-masterkube-vmware

if [ -n "${CONFIG}" ]; then
	mkdir -p "${CONFIG_DIR}"

	cp ${AUTOSCALER_VMWARE}/cluster/${CONFIG}/config ${CONFIG_DIR}/config

	cat ${AUTOSCALER_VMWARE}/config/${CONFIG}/config/kubernetes-vmware-autoscaler.json | jq \
		--arg ETCD_SSL_DIR "${AUTOSCALER_VMWARE}/cluster/${CONFIG}/etcd" \
		--arg PKI_DIR "${AUTOSCALER_VMWARE}/cluster/${CONFIG}/kubernetes/pki" \
		--arg SSH_KEY "${HOME}/.ssh/id_rsa" \
		'. | .listen = "/var/run/cluster-autoscaler/vmware.sock" | .network = "unix" | ."src-etcd-ssl-dir" = $ETCD_SSL_DIR | ."kubernetes-pki-srcdir" = $PKI_DIR | ."ssh-infos"."ssh-private-key" = $SSH_KEY' > ${CONFIG_DIR}/kubernetes-vmware-autoscaler.json
fi
