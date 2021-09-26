#!/bin/bash

set -e

CONTROL_PLANE_ENDPOINT=
CLUSTER_NODES=
NET_IP=0.0.0.0
APISERVER_ADVERTISE_PORT=6443
NGINX_CONF=/etc/nginx/tcpconf.d/apiserver.conf

TEMP=$(getopt -o l:p:n: --long listen-ip:,cluster-nodes:,control-plane-endpoint: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -p | --control-plane-endpoint)
        CONTROL_PLANE_ENDPOINT="$2"
        shift 2
        ;;
    -n | --cluster-nodes)
        CLUSTER_NODES="$2"
        shift 2
        ;;
    -l | --listen-ip)
        NET_IP="$2"
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

echo "127.0.0.1 ${CONTROL_PLANE_ENDPOINT}" >> /etc/hosts

apt install nginx -y || echo "Need to reconfigure NGINX"

# Remove IPv6 listening
sed -i '/\[::\]:80/d' /etc/nginx/sites-enabled/default

echo "include /etc/nginx/tcpconf.d/*.conf;" >> /etc/nginx/nginx.conf

mkdir -p /etc/nginx/tcpconf.d

cat > $NGINX_CONF <<EOF
stream {
    upstream kubernetes_apiserver_lb {
        least_conn;
EOF

# Buod stream TCP
IFS=, read CLUSTER_NODE <<< $CLUSTER_NODE

for CLUSTER_NODE in $(echo -n $CLUSTER_NODES | tr ',' ' ')
do
    IFS=: read HOST IP <<< $CLUSTER_NODE

    echo "$IP   $HOST" >> /etc/hosts

cat >> $NGINX_CONF <<EOF
        server ${IP}:${APISERVER_ADVERTISE_PORT} max_fails=3 fail_timeout=30s;
EOF
    done

cat >> $NGINX_CONF <<EOF
    }

EOF

cat >> $NGINX_CONF <<EOF
    server {
        listen $NET_IP:${APISERVER_ADVERTISE_PORT};

        proxy_pass kubernetes_apiserver_lb;
    }
}
EOF

apt install --fix-broken

systemctl restart nginx
systemctl disable kubelet
