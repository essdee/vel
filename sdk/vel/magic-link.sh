#!/usr/bin/env bash
# magic-link.sh — Generate a magic login link for a Vel user.
#
# Usage:
#   ./magic-link.sh <user_id> [environment]
#
# Arguments:
#   user_id       User ID from users.json (e.g., "karthi", "nithin")
#   environment   "staging" or "production" (default: staging)
#
# How it works:
#   1. Stops the Vel server (bbolt requires exclusive access)
#   2. Runs a Go program inside the framework module to call MagicLinkStore.Create()
#   3. Restarts the server
#   4. Outputs the full login URL
#
# Requirements:
#   - Go installed and in PATH (or /usr/local/go/bin)
#   - sudo access (to stop/start systemd services)
#   - Run from anywhere (paths are absolute)
#
# Key facts for agents:
#   - Magic links use ?ml_token= (NOT ?token=)
#   - Tokens are stored in sessions.db (NOT auth.db)
#   - Tokens are single-use — if you test with curl, it's consumed
#   - Generate TWO tokens if you need to test: one for testing, one for the user
#   - Telegram link previews are blocked (bot UA → 204), so sending via Telegram is safe
#   - Rate limit: 5 per hour per user (resets on server restart)

set -euo pipefail

USER_ID="${1:-}"
ENV="${2:-staging}"

if [ -z "$USER_ID" ]; then
    echo "Usage: $0 <user_id> [staging|production]"
    echo "  user_id: from users.json (e.g., karthi, nithin)"
    exit 1
fi

# Environment config
if [ "$ENV" = "production" ] || [ "$ENV" = "prod" ]; then
    VEL_DIR="/opt/vel/vel"
    DATA_DIR="/opt/vel/data"
    SERVICE="openclaw-dashboard"
    DOMAIN="w-ram.ai.essd.ee"
elif [ "$ENV" = "staging" ]; then
    VEL_DIR="/opt/vel-staging/vel"
    DATA_DIR="/opt/vel-staging/data"
    SERVICE="openclaw-dashboard-staging"
    DOMAIN="w3-ram.ai.essd.ee"
else
    echo "Unknown environment: $ENV (use staging or production)"
    exit 1
fi

export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"

# Check Go is available
if ! command -v go &>/dev/null; then
    echo "Error: Go not found in PATH"
    exit 1
fi

# Check the server is running (so we know to restart it)
WAS_RUNNING=false
if systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
    WAS_RUNNING=true
fi

# Create temporary Go file inside the module (internal package access)
TMPFILE="$VEL_DIR/_cmd_magic_link_tmp.go"

cat > "$TMPFILE" << GOEOF
package main

import (
	"fmt"
	"os"
	"time"
	bolt "go.etcd.io/bbolt"
	"vel/internal/auth"
)

func main() {
	userID := os.Args[1]
	dbPath := os.Args[2]
	domain := os.Args[3]

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ml, err := auth.NewMagicLinkStore(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating magic link store: %v\n", err)
		os.Exit(1)
	}

	token, err := ml.Create(userID, 15)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating magic link: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("https://%s/auth/magic?ml_token=%s\n", domain, token)
}
GOEOF

# Stop server (bbolt requires exclusive access)
if [ "$WAS_RUNNING" = true ]; then
    echo "Stopping $SERVICE..." >&2
    sudo systemctl stop "$SERVICE"
    sleep 1
fi

# Generate the magic link
# Note: go run with a single file in a module dir needs all files or a pattern.
# We use a subdir to avoid conflicts with the main package.
mkdir -p "$VEL_DIR/_tmp_cmd"
mv "$TMPFILE" "$VEL_DIR/_tmp_cmd/main.go"
TMPFILE="$VEL_DIR/_tmp_cmd/main.go"
cd "$VEL_DIR"
LINK=$(GOWORK=off GOTOOLCHAIN=auto go run ./_tmp_cmd "$USER_ID" "$DATA_DIR/sessions.db" "$DOMAIN" 2>&1)
EXIT_CODE=$?

# Clean up
rm -rf "$VEL_DIR/_tmp_cmd"

# Restart server
if [ "$WAS_RUNNING" = true ]; then
    echo "Starting $SERVICE..." >&2
    sudo systemctl start "$SERVICE"
fi

if [ $EXIT_CODE -ne 0 ]; then
    echo "Error generating magic link: $LINK" >&2
    exit 1
fi

echo "$LINK"
