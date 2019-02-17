#!/bin/bash
set -e

export Test_AuthMethodKey=NO
export Test_Sudo=NO
export Test_CIDR=YES
export Test_getVM=YES
export Test_listVM=YES
export Test_createVM=YES
export Test_statusVM=YES
export Test_powerOnVM=YES
export Test_powerOffVM=YES
export Test_shutdownGuest=YES
export Test_deleteVM=YES

function cleanup {
  echo "Kill vcsim"
  kill $GOVC_SIM_PID
}

trap cleanup EXIT

echo "Launch vcsim"
vcsim -pg 2 &
GOVC_SIM_PID=$!

echo "Run test"
go test --test.short -race ./vsphere
