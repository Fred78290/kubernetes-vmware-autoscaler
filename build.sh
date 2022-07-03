#!/bin/bash
sudo rm -rf out

VERSION=v1.24.2
REGISTRY=fred78290

make -e REGISTRY=$REGISTRY -e TAG=$VERSION container-push-manifest
