#!/bin/bash
# Runs vel verify periodically and sends an OpenClaw wake notification on failure.
#
# Add to crontab:
#   */15 * * * * /opt/vel/sdk/vel/verify-cron.sh
#
# Or for a specific vel install path:
#   */15 * * * * /path/to/vel/sdk/vel/verify-cron.sh

VEL_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
# Decision 016: if we landed in the framework dir (vel/), go up one more level
if [ -f "$VEL_DIR/../config/vel.json" ] || [ -d "$VEL_DIR/../vel/.git" ]; then
    VEL_DIR="$(cd "$VEL_DIR/.." && pwd)"
fi
cd "$VEL_DIR"

# Run verify in JSON mode (appends one JSONL line to logs/verify.jsonl)
# Binary is at bin/vel (Decision 016 layout) or ./vel (legacy)
VEL_BIN="./vel"
if [ -f "$VEL_DIR/bin/vel" ]; then
    VEL_BIN="./bin/vel"
fi
$VEL_BIN verify --json > /dev/null 2>&1
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
    # Read gateway token from .env
    GATEWAY_TOKEN=""
    if [ -f "$VEL_DIR/.env" ]; then
        GATEWAY_TOKEN=$(grep "^OPENCLAW_GATEWAY_TOKEN=" "$VEL_DIR/.env" | cut -d= -f2)
    fi
    GATEWAY_PORT="${OPENCLAW_GATEWAY_PORT:-18789}"

    if [ -n "$GATEWAY_TOKEN" ]; then
        VERIFY_LOG=$(tail -1 "$VEL_DIR/logs/verify.jsonl" 2>/dev/null || echo "{}")
        curl -s -X POST "http://localhost:${GATEWAY_PORT}/__openclaw__/api/cron/wake" \
            -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
            -H "Content-Type: application/json" \
            -d "{\"text\": \"vel verify cron check FAILED. Latest result: ${VERIFY_LOG}\"}" \
            > /dev/null 2>&1 || true
    fi
fi

exit $EXIT_CODE
