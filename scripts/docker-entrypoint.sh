#!/bin/sh
set -e

# Config is rendered from config.template.yml using environment only (no secrets in git).
# DATABASE_ADDRESS: Nakama/libpq form (not a full postgresql:// URL), e.g.:
#   postgres:password@postgres:5432/nakama?sslmode=disable
#   neondb_owner:YOUR_PASSWORD@ep-xxx.region.aws.neon.tech:5432/neondb?sslmode=require

if [ -z "${DATABASE_ADDRESS}" ]; then
  echo "docker-entrypoint: DATABASE_ADDRESS is required" >&2
  exit 1
fi

TEMPLATE="${NAKAMA_CONFIG_TEMPLATE:-/nakama/templates/config.template.yml}"
OUT="${NAKAMA_RUNTIME_CONFIG:-/tmp/nakama-runtime.yml}"

perl /nakama/scripts/render-config.pl "$TEMPLATE" "$OUT"

/nakama/nakama migrate up --database.address "$DATABASE_ADDRESS"
exec /nakama/nakama --config "$OUT" --database.address "$DATABASE_ADDRESS"
