#!/bin/bash
# Runs vel verify periodically and sends an OpenClaw wake notification on failure.
#
# Add to crontab:
#   */15 * * * * /opt/vel/sdk/vel/verify-cron.sh
#
# Or for a specific vel install path:
#   */15 * * * * /path/to/vel/sdk/vel/verify-cron.sh

VEL_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$VEL_DIR"

# Run verify in JSON mode (appends one JSONL line to logs/verify.jsonl)
./vel verify --json > /dev/null 2>&1
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
