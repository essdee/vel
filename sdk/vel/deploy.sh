#!/bin/bash
# Vel Deploy Script — auto-detects everything, no configuration needed.
# Called by the Velboard "Deploy" button via POST /api/updates/apply
#
# Setup: cp deploy.sh.example deploy.sh && chmod +x deploy.sh

set -e

VEL_DIR="$(cd "$(dirname "$0")" && pwd)"

# Auto-detect Go
GO="$(which go 2>/dev/null || echo /usr/local/go/bin/go)"

# Auto-detect systemd service name by finding which service runs from this dir
SERVICE_NAME=""
for svc in openclaw-dashboard openclaw-dashboard-staging; do
    unit_path=$(systemctl show "$svc" -p FragmentPath --value 2>/dev/null || true)
    if [ -n "$unit_path" ] && grep -q "$VEL_DIR" "$unit_path" 2>/dev/null; then
        SERVICE_NAME="$svc"
        break
    fi
done
# Fallback: check user services
if [ -z "$SERVICE_NAME" ]; then
    for svc in openclaw-dashboard openclaw-dashboard-staging; do
        unit_path=$(systemctl --user show "$svc" -p FragmentPath --value 2>/dev/null || true)
        if [ -n "$unit_path" ] && grep -q "$VEL_DIR" "$unit_path" 2>/dev/null; then
            SERVICE_NAME="$svc"
            break
        fi
    done
fi
if [ -z "$SERVICE_NAME" ]; then
    echo "❌ Could not auto-detect systemd service for $VEL_DIR"
    exit 1
fi

echo "⚡ Vel Deploy"
echo ""

# Step 1: Pull framework
echo "📥 Pulling vel framework..."
cd "$VEL_DIR"
git pull --ff-only 2>/dev/null || echo "  (pull failed — skipping)"

# Step 2: Pull each app
for app_dir in "$VEL_DIR"/apps/*/; do
    app_name=$(basename "$app_dir")
    if [ -d "$app_dir/.git" ]; then
        echo "📥 Pulling $app_name..."
        cd "$app_dir"
        git pull --ff-only 2>/dev/null || echo "  (pull failed for $app_name — continuing)"
    fi
done

# Step 3: Build
echo ""
echo "🔨 Building..."
cd "$VEL_DIR"
$GO run . build

# Step 4: Restart
echo ""
echo "🔄 Restarting $SERVICE_NAME..."
sudo systemctl restart "$SERVICE_NAME"
sleep 2

if sudo systemctl is-active --quiet "$SERVICE_NAME"; then
    echo ""
    echo "✅ Deploy complete!"
else
    echo ""
    echo "❌ Service failed to start!"
    sudo journalctl -u "$SERVICE_NAME" --no-pager -n 20
    exit 1
fi
