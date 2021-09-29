#!/bin/bash
APISERVER_VIP=$1
APISERVER_DEST_PORT=6443
KEEPALIVED_PASSWORD=$2
KEEPALIVED_PRIORITY=$3
KEEPALIVED_MCAST=$4
KEEPALIVED_PEER1=$5
KEEPALIVED_PEER2=$6
KEEPALIVED_STATUS=$7

if [ -f /etc/keepalived/check_apiserver.sh ]; then
    exit 0
fi

echo "net.ipv4.ip_nonlocal_bind = 1" >> /etc/sysctl.conf

apt install keepalived -y
sysctl -p

cat > /etc/keepalived/check_apiserver.sh <<EOF
#!/bin/sh

errorExit() {
    echo "*** \$*" 1>&2
    exit 1
}

curl --silent --max-time 2 --insecure https://localhost:${APISERVER_DEST_PORT}/ -o /dev/null || errorExit "Error GET https://localhost:${APISERVER_DEST_PORT}/"
if ip addr | grep -q ${APISERVER_VIP}; then
    curl --silent --max-time 2 --insecure https://${APISERVER_VIP}:${APISERVER_DEST_PORT}/ -o /dev/null || errorExit "Error GET https://${APISERVER_VIP}:${APISERVER_DEST_PORT}/"
fi
EOF

chmod +x /etc/keepalived/check_apiserver.sh

cat > /etc/keepalived/keepalived.conf <<EOF
! /etc/keepalived/keepalived.conf
! Configuration File for keepalived
global_defs {
    router_id LVS_DEVEL
    vrrp_skip_check_adv_addr
    vrrp_garp_interval 0
    vrrp_gna_interval 0
}

vrrp_script check_apiserver {
    script "/etc/keepalived/check_apiserver.sh"
    interval 3
    weight -2
    fall 10
    rise 2
}

vrrp_instance VI_1 {
    state $KEEPALIVED_STATUS
    interface eth1
    virtual_router_id 151
    priority $KEEPALIVED_PRIORITY
    advert_int 1
    unicast_src_ip $KEEPALIVED_MCAST
    authentication {
        auth_type PASS
        auth_pass $KEEPALIVED_PASSWORD
    }
    unicast_peer {
        $KEEPALIVED_PEER1
        $KEEPALIVED_PEER2
    }
    virtual_ipaddress {
        $APISERVER_VIP/24
    }
    track_script {
        check_apiserver
    }
}
EOF

systemctl daemon-reload && systemctl enable keepalived && systemctl restart keepalived
