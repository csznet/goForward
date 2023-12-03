#!/bin/bash

ARCH=$(uname -m)

if [ "$ARCH" == "x86_64" ]; then
  FILE="goForward.zip"
elif [ "$ARCH" == "aarch64" ]; then
  FILE="goForward_arm64.zip"
else
  echo -e "\e[41mError\e[0m: Unsupported architecture: $ARCH"
  exit 1
fi

# Check if unzip is installed
if ! command -v unzip &> /dev/null; then
  echo -e "\e[41mError\e[0m: unzip is not installed. Installing..."

  # Install unzip based on the package manager
  if command -v apt-get &> /dev/null; then
    sudo apt-get install -y unzip
  elif command -v yum &> /dev/null; then
    sudo yum install -y unzip
  else
    echo -e "\e[41mError\e[0m: Unsupported package manager. Please install unzip manually."
    exit 1
  fi
fi

# Download and unzip
if ! wget "https://github.com/csznet/goForward/releases/latest/download/$FILE"; then
  echo -e "\e[41mError\e[0m: Failed to download $FILE. Please check your internet connection or try again later."
  exit 1
fi

if ! unzip "$FILE"; then
  echo -e "\e[41mError\e[0m: Failed to unzip $FILE."
  exit 1
fi

rm "$FILE"

# Set permissions
chmod +x goForward

# Output success message
echo -e "\e[44mSuccess\e[0m: The 'goForward' executable is ready for use."
