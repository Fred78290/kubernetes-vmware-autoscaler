#/bin/bash

protoc -I . -I vendor grpc/grpc.proto --go_out=plugins=grpc:.
