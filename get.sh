#!/bin/bash

ARCH=$(uname -m)

if [ "$ARCH" == "x86_64" ]; then
  File="goForward.zip"
elif [ "$ARCH" == "aarch64" ]; then
  File="goForward_arm64.zip"
else
  echo "Unsupported architecture: $ARCH"
  exit 1
fi

# Check if unzip is installed
if ! command -v unzip &> /dev/null; then
  echo "unzip is not installed. Installing..."
  
  # Install unzip based on the package manager
  if command -v apt-get &> /dev/null; then
    sudo apt-get install -y unzip
  elif command -v yum &> /dev/null; then
    sudo yum install -y unzip
  else
    echo "Unsupported package manager. Please install unzip manually."
    exit 1
  fi
fi

# Download and unzip
wget "https://github.com/csznet/goForward/releases/latest/download/$File" && unzip "$File" && rm "$File"

# Set permissions
chmod +x goForward

echo "Suc!"
