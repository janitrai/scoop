#!/usr/bin/env bash
set -euo pipefail

# Setup systemd services for scoop backend + frontend
# Usage: sudo ./scripts/setup-systemd.sh [--user bob] [--backend-port 8090] [--frontend-port 5173]

SCOOP_USER="${USER:-bob}"
BACKEND_PORT=8090
FRONTEND_PORT=5173
SCOOP_DIR="$(cd "$(dirname "$0")/.." && pwd)"

while [[ $# -gt 0 ]]; do
  case $1 in
    --user) SCOOP_USER="$2"; shift 2;;
    --backend-port) BACKEND_PORT="$2"; shift 2;;
    --frontend-port) FRONTEND_PORT="$2"; shift 2;;
    *) echo "Unknown flag: $1"; exit 1;;
  esac
done

echo "==> Building scoop binary..."
cd "$SCOOP_DIR/backend"
go build -o /tmp/scoop ./cmd/scoop/
sudo cp /tmp/scoop /usr/local/bin/scoop
echo "    Installed /usr/local/bin/scoop"

# Detect node/pnpm path
NODE_BIN="$(dirname "$(which node)")"
echo "    Node bin: $NODE_BIN"

echo "==> Writing scoop-serve.service..."
sudo tee /etc/systemd/system/scoop-serve.service > /dev/null <<EOF
[Unit]
Description=Scoop News Pipeline API Server
After=network.target postgresql.service

[Service]
Type=simple
User=$SCOOP_USER
WorkingDirectory=$SCOOP_DIR/backend
ExecStart=/usr/local/bin/scoop serve --host 0.0.0.0 --port $BACKEND_PORT
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

echo "==> Writing scoop-frontend.service..."
sudo tee /etc/systemd/system/scoop-frontend.service > /dev/null <<EOF
[Unit]
Description=Scoop Frontend Dev Server
After=network.target scoop-serve.service

[Service]
Type=simple
User=$SCOOP_USER
WorkingDirectory=$SCOOP_DIR/frontend
Environment=PATH=$NODE_BIN:/usr/local/bin:/usr/bin:/bin
ExecStart=$NODE_BIN/pnpm run dev --host 0.0.0.0 --port $FRONTEND_PORT --strictPort
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

echo "==> Reloading systemd..."
sudo systemctl daemon-reload

echo "==> Enabling services..."
sudo systemctl enable scoop-serve scoop-frontend

echo "==> Starting services..."
sudo systemctl restart scoop-serve
sleep 2
sudo systemctl restart scoop-frontend
sleep 2

echo "==> Status:"
echo "    scoop-serve:    $(systemctl is-active scoop-serve)"
echo "    scoop-frontend: $(systemctl is-active scoop-frontend)"

echo ""
echo "Done. Useful commands:"
echo "  sudo systemctl status scoop-serve"
echo "  sudo systemctl status scoop-frontend"
echo "  sudo journalctl -u scoop-serve -f"
echo "  sudo journalctl -u scoop-frontend -f"
