#!/bin/bash
set -e

GO="/home/bedri/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64/bin/go"

if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
  echo "==> .env created from .env.example (edit it to add API keys)"
fi

echo "==> Running tests..."
$GO test ./... || { echo "Tests failed"; exit 1; }

echo "==> Building..."
$GO build -o odk . || { echo "Build failed"; exit 1; }

echo "==> Stopping agent..."
systemctl --user stop odk-agent 2>/dev/null || true

echo "==> Installing..."
chmod +x odk

echo "==> Starting agent..."
systemctl --user start odk-agent

echo "==> Done"
systemctl --user status odk-agent --no-pager -l 2>&1 | head -5
