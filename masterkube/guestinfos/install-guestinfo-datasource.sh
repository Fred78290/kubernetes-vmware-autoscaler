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

shutdown -P now
