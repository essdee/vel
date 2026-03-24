#!/bin/bash
# Vel Deploy Script — auto-detects everything, no configuration needed.
# Called by the Velboard "Deploy" button via POST /api/updates/apply
#
# Setup: cp deploy.sh.example deploy.sh && chmod +x deploy.sh

set -e

# Resolve to project root: script is at sdk/vel/deploy.sh
# With symlink: 2 levels up from project/sdk/vel/ = project root
# Without symlink: 2 levels up from project/vel/sdk/vel/ = framework dir (vel/)
# Detect and correct: if we landed in the framework dir, go up one more level
VEL_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
if [ -f "$VEL_DIR/../config/vel.json" ] || [ -d "$VEL_DIR/../vel/.git" ]; then
    VEL_DIR="$(cd "$VEL_DIR/.." && pwd)"
fi

# Auto-detect Go
GO="$(which go 2>/dev/null || echo /usr/local/go/bin/go)"

# Load .env for optional variables
if [ -f "$VEL_DIR/.env" ]; then
    while IFS='=' read -r key value; do
        case "$key" in
            OPENCLAW_GATEWAY_TOKEN) OPENCLAW_GATEWAY_TOKEN="${value}" ;;
            OPENCLAW_GATEWAY_PORT) OPENCLAW_GATEWAY_PORT="${value}" ;;
            VEL_APPS) VEL_APPS="${value}" ;;
            VEL_SERVICE_NAME) VEL_SERVICE_NAME="${value}" ;;
        esac
    done < "$VEL_DIR/.env"
fi
OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-}"
OPENCLAW_GATEWAY_PORT="${OPENCLAW_GATEWAY_PORT:-18789}"
VEL_APPS="${VEL_APPS:-}"

# Determine systemd service name
# Priority: VEL_SERVICE_NAME env var > auto-detect > default
if [ -n "${VEL_SERVICE_NAME:-}" ]; then
    SERVICE_NAME="$VEL_SERVICE_NAME"
else
    # Auto-detect systemd service name by finding which service runs from this dir
    SERVICE_NAME=""
    for svc in vel vel-staging openclaw-dashboard openclaw-dashboard-staging; do
        unit_path=$(systemctl show "$svc" -p FragmentPath --value 2>/dev/null || true)
        if [ -n "$unit_path" ] && grep -q "$VEL_DIR" "$unit_path" 2>/dev/null; then
            SERVICE_NAME="$svc"
            break
        fi
    done
    # Fallback: check user services
    if [ -z "$SERVICE_NAME" ]; then
        for svc in vel vel-staging openclaw-dashboard openclaw-dashboard-staging; do
            unit_path=$(systemctl --user show "$svc" -p FragmentPath --value 2>/dev/null || true)
            if [ -n "$unit_path" ] && grep -q "$VEL_DIR" "$unit_path" 2>/dev/null; then
                SERVICE_NAME="$svc"
                break
            fi
        done
    fi
    if [ -z "$SERVICE_NAME" ]; then
        echo "❌ Could not auto-detect systemd service for $VEL_DIR"
        echo "   Set VEL_SERVICE_NAME env var (e.g., vel or vel-staging)"
        exit 1
    fi
fi

echo "⚡ Vel Deploy — $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo "   VEL_DIR: $VEL_DIR"
echo "   SERVICE: $SERVICE_NAME"
echo ""

# Helper: stash-aware git pull for a repo directory
# Handles local modifications that would block --ff-only
safe_git_pull() {
    local repo_dir="$1"
    local repo_name="$2"
    cd "$repo_dir"

    # Check for local modifications
    local dirty=$(git status --porcelain 2>/dev/null | grep -v '^??' || true)
    local stashed=false

    if [ -n "$dirty" ]; then
        echo "  ⚠ $repo_name has local changes — stashing..."
        echo "$dirty" | sed 's/^/    /'
        if git stash push -m "vel-deploy-$(date +%Y%m%d-%H%M%S)" 2>/dev/null; then
            stashed=true
        else
            echo "  ❌ $repo_name: stash failed — skipping pull"
            return 1
        fi
    fi

    # Pull
    if git pull --ff-only 2>&1; then
        echo "  ✅ $repo_name: pulled"
    else
        echo "  ❌ $repo_name: pull failed"
        # Pop stash back if we stashed
        if $stashed; then
            git stash pop 2>/dev/null || true
        fi
        return 1
    fi

    # Re-apply stashed changes
    if $stashed; then
        if git stash pop 2>/dev/null; then
            echo "  ✅ $repo_name: local changes re-applied"
        else
            echo "  ⚠ $repo_name: stash pop conflict — local changes in stash, resolve manually"
            echo "    Run: cd $repo_dir && git stash show && git stash pop"
        fi
    fi
    return 0
}

# Step 1: Pull framework
echo "📥 Pulling vel framework..."
# Decision 016: framework repo may be in vel/ subdirectory
if [ -d "$VEL_DIR/vel/.git" ]; then
    safe_git_pull "$VEL_DIR/vel" "vel-framework"
else
    safe_git_pull "$VEL_DIR" "vel-framework"
fi

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
            safe_git_pull "$app_dir" "$app_name"
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
# Decision 016: build from framework dir (vel/) if it exists
if [ -f "$VEL_DIR/vel/go.mod" ]; then
    cd "$VEL_DIR/vel"
else
    cd "$VEL_DIR"
fi
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
./bin/vel verify --json 2>&1 | tee /dev/stderr

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
