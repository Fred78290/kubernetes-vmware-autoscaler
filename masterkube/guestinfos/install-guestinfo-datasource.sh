#!/bin/sh

#
# usage: install.sh
#        curl -sSL https://raw.githubusercontent.com/Fred78290/cloud-init-vmware-guestinfo/master/install.sh | sh -
#

# The script to lookup the path to the cloud-init's datasource directory, "sources".
apt-get update
apt-get upgrade -y
apt-get dist-upgrade -y

PY_SCRIPT='import os; from cloudinit import sources; print(os.path.dirname(sources.__file__));'

PYTHON=$(command -v python)

# Ensure pip installed

if [ -z "$PYTHON" ]; then
    PYTHON=$(command -v python3)
    apt install python3-pip -y
else
    apt install python-pip -y
fi

# Get the path to the cloud-init installation's datasource directory.
CLOUD_INIT_SOURCES=$($PYTHON -c ''"${PY_SCRIPT}"'' 2>/dev/null || (exit_code="${?}"; echo "failed to find python runtime" 1>&2; exit "${exit_code}"; ))

# If no "sources" directory was located then it's likely cloud-init is not installed.
[ -z "${CLOUD_INIT_SOURCES}" ] && echo "cloud-init not found" 1>&2 && exit 1

echo "Cloud init sources located here: $CLOUD_INIT_SOURCES"

# The repository from which to fetch the cloud-init datasource and config files.
REPO_SLUG="${REPO_SLUG:-https://raw.githubusercontent.com/Fred78290/cloud-init-vmware-guestinfo}"

# The git reference to use. This can be a branch or tag name as well as a commit ID.
GIT_REF="${GIT_REF:-master}"

# Download the cloud init datasource into the cloud-init's "sources" directory.
curl -sSL -o "${CLOUD_INIT_SOURCES}/DataSourceVMwareGuestInfo.py" "${REPO_SLUG}/${GIT_REF}/DataSourceVMwareGuestInfo.py"

# Add the configuration file that tells cloud-init what datasource to use.
mkdir -p /etc/cloud/cloud.cfg.d
curl -sSL -o /etc/cloud/cloud.cfg.d/99-DataSourceVMwareGuestInfo.cfg "${REPO_SLUG}/${GIT_REF}/99-DataSourceVMwareGuestInfo.cfg"

sed -i 's/None/None, VMwareGuestInfo/g' /etc/cloud/cloud.cfg.d/90_dpkg.cfg

[ -f /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg ] && rm /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg
cloud-init clean
rm /var/log/cloud-ini*

sed -i 's/^GRUB_CMDLINE_LINUX=\"/GRUB_CMDLINE_LINUX=\"ipv6.disable=1 net.ifnames=0 biosdevname=0/g' /etc/default/grub
update-grub2

cat > /lib/systemd/system/systemd-machine-id.service <<EOF
#  This file is part of systemd.
#
#  systemd is free software; you can redistribute it and/or modify it
#  under the terms of the GNU Lesser General Public License as published by
#  the Free Software Foundation; either version 2.1 of the License, or
#  (at your option) any later version.

[Unit]
Description=Regenerate machine-id if missing
Documentation=man:systemd-machine-id(1)
DefaultDependencies=no
Conflicts=shutdown.target
After=systemd-remount-fs.service
Before=systemd-sysusers.service sysinit.target shutdown.target
ConditionPathIsReadWrite=/etc
ConditionFirstBoot=yes

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/systemd-machine-id-setup
StandardOutput=tty
StandardInput=tty
StandardError=tty

[Install]
WantedBy=sysinit.target
EOF

chown root:root /lib/systemd/system/systemd-machine-id.service

systemctl enable systemd-machine-id.service

cat > /usr/local/bin/cloud-init-clean.sh <<EOF
#!/bin/bash
[ -f /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg ] && rm /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg
rm /etc/netplan/*
rm /etc/machine-id
cloud-init clean
rm /var/log/cloud-ini*
rm /var/log/syslog
shutdown -P now
EOF

chmod +x /usr/local/bin/cloud-init-clean.sh
chown root:root /usr/local/bin/cloud-init-clean.sh

/usr/local/bin/cloud-init-clean.sh
