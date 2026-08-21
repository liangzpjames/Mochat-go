#!/bin/bash
# MoChat 服务器初始化脚本：安装 Docker + 配置镜像加速
set -e

echo "===== [1/5] 更新系统软件源 ====="
export DEBIAN_FRONTEND=noninteractive
apt-get update -y -qq || echo "apt update 有警告，继续"

echo "===== [2/5] 安装 Docker 依赖 ====="
apt-get install -y -qq ca-certificates curl gnupg lsb-release git unzip >/dev/null 2>&1 || true

echo "===== [3/5] 添加 Docker 官方源（阿里云镜像） ====="
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null || {
  echo "阿里云 GPG 失败，尝试官方源"
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
}
chmod a+r /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | tee /etc/apt/sources.list.d/docker.list >/dev/null

echo "===== [4/5] 安装 Docker ====="
apt-get update -y -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null 2>&1 || {
  echo "批量安装失败，逐个重试"
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io
  apt-get install -y -qq docker-buildx-plugin docker-compose-plugin
}

echo "===== [5/5] 配置镜像加速 + 开机自启 ====="
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://dockerproxy.com",
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com"
  ],
  "log-driver": "json-file",
  "log-opts": { "max-size": "50m", "max-file": "3" }
}
EOF
systemctl daemon-reload
systemctl enable docker >/dev/null 2>&1 || true
systemctl restart docker

echo "===== 验证 ====="
docker --version
docker compose version || docker-compose --version
echo "Docker 安装完成！"
