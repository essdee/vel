#!/usr/bin/env bash
# SDK: Restart the OpenClaw gateway service.
# Called by Vel's /api/gateway/restart endpoint.
set -euo pipefail

export PATH="$HOME/.npm-global/bin:$HOME/.local/bin:$HOME/.nvm/current/bin:/usr/local/bin:/usr/bin:/bin:$PATH"

# User systemd needs the bus address
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
export DBUS_SESSION_BUS_ADDRESS="${DBUS_SESSION_BUS_ADDRESS:-unix:path=$XDG_RUNTIME_DIR/bus}"

SERVICE="openclaw-gateway.service"

# Check if service exists
if ! systemctl --user cat "$SERVICE" &>/dev/null; then
  echo "ERROR: $SERVICE not found" >&2
  exit 1
fi

echo "Restarting $SERVICE..."
systemctl --user restart "$SERVICE" 2>&1

# Wait for it to come up
sleep 3

STATUS=$(systemctl --user is-active "$SERVICE" 2>/dev/null || true)
if [[ "$STATUS" == "active" ]]; then
  PID=$(systemctl --user show -p MainPID "$SERVICE" 2>/dev/null | cut -d= -f2)
  echo "✓ Gateway restarted successfully (PID: $PID)"
else
  echo "✗ Gateway failed to start (status: $STATUS)" >&2
  systemctl --user status "$SERVICE" --no-pager 2>&1 | tail -10
  exit 1
fi
