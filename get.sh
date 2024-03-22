#!/bin/bash

ARCH=$(uname -m)

if [ "$ARCH" == "x86_64" ]; then
  FILE="goForward.zip"
elif [ "$ARCH" == "arm64" ]; then
  FILE="goForward_arm64.zip"
else
  echo -e "\033[41mError\033[0m: Unsupported architecture: $ARCH"
  exit 1
fi

# Check if unzip is installed
if ! command -v unzip &> /dev/null; then
  echo -e "\033[41mError\033[0m: unzip is not installed. Installing..."

  # Install unzip based on the package manager
  if command -v apt-get &> /dev/null; then
    sudo apt-get install -y unzip
  elif command -v yum &> /dev/null; then
    sudo yum install -y unzip
  else
    echo -e "\033[41mError\033[0m: Unsupported package manager. Please install unzip manually."
    exit 1
  fi
fi

# 获取百度的平均延迟（ping 5次并取平均值）
ping_result=$(ping -c 5 -q baidu.com | awk -F'/' 'END{print $5}')

# 判断延迟是否在100以内
if awk -v ping="$ping_result" 'BEGIN{exit !(ping < 100)}'; then
  echo "服务器位于中国国内，使用代理下载"
  url="https://mirror.ghproxy.com/https://github.com/csznet/goForward/releases/latest/download/${FILE}"
else
  echo "服务器位于国外，不使用代理下载"
  url="https://github.com/csznet/goForward/releases/latest/download/${FILE}"
fi

# Download and unzip
if ! curl -L -O $url; then
  echo -e "\033[41mError\033[0m: Failed to download $FILE. Please check your internet connection or try again later."
  exit 1
fi

if ! unzip "$FILE"; then
  echo -e "\033[41mError\033[0m: Failed to unzip $FILE."
  exit 1
fi

rm "$FILE"

# Set permissions
chmod +x goForward

# Output success message
echo -e "\033[44mSuccess\033[0m The 'goForward' executable is ready for use."