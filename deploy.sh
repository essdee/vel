#!/bin/bash
# Vel Production Deploy Script
# Pulls from Git, builds, and restarts.

set -e

PROD_DIR="/opt/vel"
GO="/usr/local/go/bin/go"

echo "⚡ Vel Deploy"
echo ""

# Step 1: Pull latest from Git (vel framework)
echo "📥 Pulling vel framework..."
cd "$PROD_DIR"
git pull --ff-only 2>/dev/null || echo "  (pull failed or not a git repo — skipping)"

# Step 2: Pull latest for each app
for app_dir in "$PROD_DIR"/apps/*/; do
    app_name=$(basename "$app_dir")
    if [ -d "$app_dir/.git" ]; then
        echo "📥 Pulling $app_name..."
        cd "$app_dir"
        # Try main first, then master
        git pull --ff-only 2>/dev/null || echo "  (pull failed for $app_name — continuing)"
    fi
done

# Step 3: Build
echo ""
echo "🔨 Building..."
cd "$PROD_DIR"
$GO run . build

# Step 4: Restart
echo ""
echo "🔄 Restarting dashboard..."
sudo systemctl restart openclaw-dashboard
sleep 2

if sudo systemctl is-active --quiet openclaw-dashboard; then
    echo ""
    echo "✅ Deploy complete!"
else
    echo ""
    echo "❌ Service failed to start!"
    sudo journalctl -u openclaw-dashboard --no-pager -n 20
    exit 1
fi
