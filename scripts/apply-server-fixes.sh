#!/bin/bash
# Run on the Nakama EC2: cd ~/lila-server && bash scripts/apply-server-fixes.sh
set -euo pipefail
cd "$(dirname "$0")/.."
git pull --ff-only
bash scripts/patch-nginx-nakama.sh
sudo docker compose -f docker-compose.prod.yml up -d --build
sleep 6
echo "=== Direct :7350 ==="
./scripts/verify-apis.sh
echo "=== Via nginx :80 ==="
NAKAMA_HOST=127.0.0.1 NAKAMA_PORT=80 ./scripts/verify-apis.sh
echo "Done."
