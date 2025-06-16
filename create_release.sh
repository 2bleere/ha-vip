#!/bin/bash

# Script to create a release package for HA VIP Manager
# Usage: ./create_release.sh <version>

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 1.0.0"
    exit 1
fi

VERSION=$1
RELEASE_DIR="releases/ha-vip-$VERSION"
ARCH=$(uname -m)

# Create release directory structure
mkdir -p "$RELEASE_DIR"

# Build for current architecture
echo "Building HA VIP Manager v$VERSION for $ARCH..."
go build -o "$RELEASE_DIR/ha-vip" -ldflags "-X main.version=$VERSION" .

# Copy configuration files
cp config.yaml "$RELEASE_DIR/"
cp cert.pem key.pem "$RELEASE_DIR/"
cp ha-vip.service "$RELEASE_DIR/"
cp setup_ha_vip.sh "$RELEASE_DIR/"
cp README.md "$RELEASE_DIR/"

# Create archive
cd releases
tar -czvf "ha-vip-$VERSION-$ARCH.tar.gz" "ha-vip-$VERSION"
cd ..

echo "Release package created: releases/ha-vip-$VERSION-$ARCH.tar.gz"

# Create cross-compiled ARM64 version if on x86_64
if [ "$ARCH" = "x86_64" ]; then
    echo "Cross-compiling for ARM64..."
    GOOS=linux GOARCH=arm64 go build -o "$RELEASE_DIR/ha-vip-linux-arm64" -ldflags "-X main.version=$VERSION" .
    
    # Create ARM64-specific archive
    mkdir -p "$RELEASE_DIR-arm64"
    cp "$RELEASE_DIR/ha-vip-linux-arm64" "$RELEASE_DIR-arm64/"
    cp "$RELEASE_DIR"/{config.yaml,cert.pem,key.pem,ha-vip.service,setup_ha_vip.sh,README.md} "$RELEASE_DIR-arm64/"
    
    cd releases
    tar -czvf "ha-vip-$VERSION-arm64.tar.gz" "ha-vip-$VERSION-arm64"
    cd ..
    
    echo "ARM64 release package created: releases/ha-vip-$VERSION-arm64.tar.gz"
fi

echo "Release v$VERSION completed successfully!"
