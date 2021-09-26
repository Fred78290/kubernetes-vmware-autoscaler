#!/bin/bash
TEMP=$(getopt -o c: --long cluster-nodes: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -c|--cluster-nodes)
        CLUSTER_NODES="$2"
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

ETCDIPS=()
ETCDHOSTS=()
ETCDNAMES=()

for CLUSTER_NODE in $(echo -n ${CLUSTER_NODES} | tr ',' ' ')
do
    IFS=: read HOST IP <<< ${CLUSTER_NODE}

    ETCDIPS+=($IP)
    ETCDHOSTS+=($HOST)
    ETCDNAMES+=(${HOST%%.*})
done

mkdir -p ./cluster/etcd/ssl

cat > ./cluster/etcd/ca-config.json <<EOF
{
  "signing": {
    "default": {
      "expiry": "87600h"
    },
    "profiles": {
      "kubernetes": {
        "usages": [
            "signing",
            "key encipherment",
            "server auth",
            "client auth"
        ],
        "expiry": "87600h"
      }
    }
  }
}
EOF

cat > ./cluster/etcd/ca-csr.json <<EOF
{
  "CN": "kubernetes",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
        "C": "FR",
        "ST": "PARIS",
        "L": "Paris",
        "O": "k8s",
        "OU": "System"
    }
  ]
}
EOF

cat > ./cluster/etcd/etcd-csr.json <<EOF
{
    "CN": "etcd",
    "hosts": [
        "127.0.0.1",
        "${ETCDIPS[0]}",
        "${ETCDIPS[1]}",
        "${ETCDIPS[2]}",
        "${ETCDHOSTS[0]}",
        "${ETCDHOSTS[1]}",
        "${ETCDHOSTS[2]}",
        "${ETCDNAMES[0]}",
        "${ETCDNAMES[1]}",
        "${ETCDNAMES[2]}"
    ],
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "FR",
            "ST": "PARIS",
            "L": "Paris",
            "O": "k8s",
            "OU": "System"
        }
    ]
}
EOF

pushd ./cluster/etcd/
cfssl gencert -initca ca-csr.json | cfssljson -bare ./ssl/ca
cfssl gencert -ca=./ssl/ca.pem -ca-key=./ssl/ca-key.pem -config=ca-config.json -profile=kubernetes etcd-csr.json | cfssljson -bare ./ssl/etcd
popd

for INDEX in "${!ETCDHOSTS[@]}";
do
    echo "Generate etcd config index: $INDEX"

    IP=${ETCDIPS[$INDEX]}
    HOST=${ETCDHOSTS[$INDEX]}
    NAME=${ETCDNAMES[$INDEX]}
    ETCINDEX="0$((INDEX+1))"
    SERVICE=./cluster/etcd/etcd-${ETCINDEX}.service

    cat > ${SERVICE} << EOF
[Unit]
Description=Etcd Server
After=network.target
After=network-online.target
Wants=network-online.target
Documentation=https://github.com/coreos
[Service]
Type=notify
WorkingDirectory=/var/lib/etcd/
ExecStart=/usr/local/bin/etcd \
    --name=${NAME} \
    --advertise-client-urls=https://${IP}:2379 \
    --cert-file=/etc/etcd/ssl/etcd.pem \
    --key-file=/etc/etcd/ssl/etcd-key.pem \
    --peer-cert-file=/etc/etcd/ssl/etcd.pem \
    --peer-key-file=/etc/etcd/ssl/etcd-key.pem \
    --trusted-ca-file=/etc/etcd/ssl/ca.pem \
    --peer-trusted-ca-file=/etc/etcd/ssl/ca.pem \
    --data-dir=/var/lib/etcd \
    --initial-advertise-peer-urls=https://${IP}:2380 \
    --initial-cluster-state=new \
    --initial-cluster-token=etcd-cluster-0 \
    --initial-cluster=${ETCDNAMES[0]}=https://${ETCDIPS[0]}:2380,${ETCDNAMES[1]}=https://${ETCDIPS[1]}:2380,${ETCDNAMES[2]}=https://${ETCDIPS[2]}:2380 \
    --listen-client-urls=https://${IP}:2379,http://127.0.0.1:2379 \
    --listen-metrics-urls=http://127.0.0.1:2381 \
    --listen-peer-urls=https://${IP}:2380
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
EOF
done
