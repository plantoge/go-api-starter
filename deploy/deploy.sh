#!/usr/bin/env bash
set -euo pipefail

cd /opt/app

echo "==> pulling latest code"
git pull

echo "==> backing up current binaries"
cp -f api api.backup 2>/dev/null || true
cp -f cli cli.backup 2>/dev/null || true

echo "==> building"
go build -o api ./cmd/api
go build -o cli ./cmd/cli

echo "==> migrating platform schema"
./cli migrate platform up

echo "==> migrating tenant schemas"
./cli migrate tenant up --all

echo "==> syncing permissions"
./cli permission sync --all

echo "==> restarting service"
sudo systemctl restart app-api

echo "==> done"
