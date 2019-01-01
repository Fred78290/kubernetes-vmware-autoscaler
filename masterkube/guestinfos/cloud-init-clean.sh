#!/bin/bash

rm /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg
cloud-init clean
rm /var/log/cloud-ini*
shutdown now
