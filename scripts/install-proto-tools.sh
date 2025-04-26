#!/bin/bash

set -e

echo "Installing Protocol Buffer tools..."

# Install protoc
if ! command -v protoc &> /dev/null; then
    echo "Installing protoc..."
    # This is a simple example, you might want to adapt this for different platforms
    PROTOC_VERSION="3.20.3"
    PROTOC_ZIP="protoc-${PROTOC_VERSION}-osx-x86_64.zip"  # Change this for different platforms
    curl -OL "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${PROTOC_ZIP}"
    sudo unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
    sudo unzip -o $PROTOC_ZIP -d /usr/local 'include/*'
    rm -f $PROTOC_ZIP
else
    echo "protoc is already installed"
fi

# Install Go plugins for protoc
echo "Installing Go plugins for Protocol Buffers..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

# Install gRPC Gateway plugins for REST
echo "Installing gRPC Gateway plugins..."
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.15.2
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.15.2

echo "Protocol Buffer tools installation complete" 