#!/bin/bash
# Amazon Linux 2023 — Nakama (Docker) + nginx :80 -> :7350
# Run as ec2-user with sudo. Set NEON_DATABASE_ADDRESS before running or edit ~/lila-server/.env after.

set -euo pipefail

PUBLIC_IP="${PUBLIC_IP:-54.160.155.191}"
NAKAMA_KEY="${NAKAMA_KEY:-nebula-strike-prod-key}"

echo "== Packages: git, nginx, docker =="
sudo dnf -y update
sudo dnf install -y git nginx docker

if sudo dnf install -y docker-compose-plugin 2>/dev/null; then
  echo "docker-compose-plugin installed via dnf"
else
  echo "Installing docker compose v2 plugin manually..."
  mkdir -p ~/.docker/cli-plugins
  curl -fsSL "https://github.com/docker/compose/releases/download/v2.29.7/docker-compose-linux-x86_64" \
    -o ~/.docker/cli-plugins/docker-compose
  chmod +x ~/.docker/cli-plugins/docker-compose
  sudo mkdir -p /usr/local/lib/docker/cli-plugins
  sudo ln -sf ~/.docker/cli-plugins/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose 2>/dev/null || true
fi

sudo systemctl enable --now docker
sudo usermod -aG docker ec2-user || true

echo "== nginx base config (only conf.d servers; avoids duplicate listen 80) =="
sudo tee /etc/nginx/nginx.conf >/dev/null <<'MAIN'
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log notice;
pid /run/nginx.pid;

include /usr/share/nginx/modules/*.conf;

events {
    worker_connections 1024;
}

http {
    log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"';

    access_log  /var/log/nginx/access.log  main;

    sendfile            on;
    tcp_nopush          on;
    keepalive_timeout   65;
    types_hash_max_size 4096;

    include             /etc/nginx/mime.types;
    default_type        application/octet-stream;

    # HTTP API + WebSocket on same port: only WebSocket requests send Upgrade; do not force Connection: upgrade for RPC/REST.
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    include /etc/nginx/conf.d/*.conf;
}
MAIN

echo "== nginx reverse proxy :80 -> Nakama :7350 =="
sudo tee /etc/nginx/conf.d/nakama.conf >/dev/null <<'NGINX'
server {
    listen 80 default_server;
    server_name _;
    location / {
        proxy_pass http://127.0.0.1:7350;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
NGINX

sudo rm -f /etc/nginx/conf.d/default.conf 2>/dev/null || true
sudo nginx -t
sudo systemctl enable nginx
sudo systemctl restart nginx

if command -v getenforce >/dev/null 2>&1 && [ "$(getenforce 2>/dev/null)" = "Enforcing" ]; then
  sudo setsebool -P httpd_can_network_connect 1 2>/dev/null || true
fi

echo "== git identity =="
git config --global user.name "yash717"
git config --global user.email "yash717@users.noreply.github.com"

echo "== Clone / update lila-server =="
cd /home/ec2-user
if [ ! -d lila-server ]; then
  git clone https://github.com/yash717/lila-server.git
fi
cd lila-server
git pull --ff-only

echo "== .env (set DATABASE_ADDRESS to Neon Nakama form) =="
if [ ! -f .env ]; then
  CONSOLE_PW=$(openssl rand -hex 12)
  cat > .env << EOF
# Nakama DB — replace with your Neon connection in Nakama form:
# user:pass@host:5432/dbname?sslmode=require
DATABASE_ADDRESS=${NEON_DATABASE_ADDRESS:-REPLACE_ME_SET_NEON_HERE}

NAKAMA_SERVER_KEY=${NAKAMA_KEY}
NAKAMA_CONSOLE_USERNAME=admin
NAKAMA_CONSOLE_PASSWORD=${CONSOLE_PW}
EOF
  echo "Created .env — edit DATABASE_ADDRESS if still REPLACE_ME_SET_NEON_HERE"
  echo "Console password (save securely): ${CONSOLE_PW}"
else
  echo ".env already exists; not overwriting"
fi

echo "== Docker Compose (production) =="
if grep -q 'REPLACE_ME_SET_NEON_HERE' .env 2>/dev/null; then
  echo "WARNING: DATABASE_ADDRESS not set. Skipping docker compose until you edit ~/lila-server/.env"
  echo "Then run: cd ~/lila-server && docker compose -f docker-compose.prod.yml up -d --build"
else
  # Same shell may not have docker group yet
  sudo docker compose -f docker-compose.prod.yml up -d --build
fi

echo "== Done. Nakama API (via nginx): http://${PUBLIC_IP}/"
echo "Console (open SG for 7351): http://${PUBLIC_IP}:7351/"
