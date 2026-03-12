#!/bin/bash
# Vel Deploy Script — auto-detects everything, no configuration needed.
# Called by the Velboard "Deploy" button via POST /api/updates/apply
#
# Setup: cp deploy.sh.example deploy.sh && chmod +x deploy.sh

set -e

# Resolve to Vel root: script is at sdk/vel/deploy.sh, so root is 2 levels up
VEL_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

# Auto-detect Go
GO="$(which go 2>/dev/null || echo /usr/local/go/bin/go)"

# Load .env for optional variables
if [ -f "$VEL_DIR/.env" ]; then
    while IFS='=' read -r key value; do
        case "$key" in
            OPENCLAW_GATEWAY_TOKEN) OPENCLAW_GATEWAY_TOKEN="${value}" ;;
            OPENCLAW_GATEWAY_PORT) OPENCLAW_GATEWAY_PORT="${value}" ;;
            VEL_APPS) VEL_APPS="${value}" ;;
        esac
    done < "$VEL_DIR/.env"
fi
OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-}"
OPENCLAW_GATEWAY_PORT="${OPENCLAW_GATEWAY_PORT:-18789}"
VEL_APPS="${VEL_APPS:-}"

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
# Helper: pull git-tracked apps from a given directory
pull_apps_from_dir() {
    local dir="$1"
    [ -d "$dir" ] || return 0
    for app_dir in "$dir"/*/; do
        [ -d "$app_dir" ] || continue
        app_name=$(basename "$app_dir")
        if [ -d "$app_dir/.git" ]; then
            echo "📥 Pulling $app_name..."
            cd "$app_dir"
            git pull --ff-only 2>/dev/null || echo "  (pull failed for $app_name — continuing)"
        fi
    done
}

# Pull from built-in apps/ directory (backward compat)
pull_apps_from_dir "$VEL_DIR/apps"

# Pull from external VEL_APPS directory (if configured)
if [ -n "$VEL_APPS" ]; then
    pull_apps_from_dir "$VEL_APPS"
fi

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
    echo "✅ Service restarted successfully"
else
    echo ""
    echo "❌ Service failed to start!"
    sudo journalctl -u "$SERVICE_NAME" --no-pager -n 20
    exit 1
fi

# Step 5: Verify
echo ""
echo "🔍 Verifying deployment..."
sleep 2  # give server a moment to fully initialize
cd "$VEL_DIR"
./vel verify --json 2>&1 | tee /dev/stderr

if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo ""
    echo "❌ Verify failed! Check logs/verify.jsonl for details."

    # Notify OpenClaw agent if gateway is available
    if [ -n "$OPENCLAW_GATEWAY_TOKEN" ]; then
        VERIFY_LOG=$(cat "$VEL_DIR/logs/verify.jsonl" 2>/dev/null || echo "{}")
        curl -s -X POST "http://localhost:${OPENCLAW_GATEWAY_PORT}/__openclaw__/api/cron/wake" \
            -H "Authorization: Bearer ${OPENCLAW_GATEWAY_TOKEN}" \
            -H "Content-Type: application/json" \
            -d "{\"text\": \"vel verify failed after deploy. Failures: ${VERIFY_LOG}\"}" \
            >/dev/null 2>&1 || true
        echo "📡 Notified OpenClaw agent about failures"
    fi
    exit 1
fi

echo ""
echo "✅ Deploy complete!"
