#!/bin/bash
# Vel verify cron — runs twice daily (8 AM, 8 PM IST)
# Alerts the agent only on failure.

export PATH=$PATH:/usr/local/go/bin:/home/claw/go/bin

# Detect environment
if [ -d "/opt/vel-staging" ] && [ -f "/opt/vel-staging/bin/vel" ]; then
    VEL_DIR="/opt/vel-staging"
    ENV="staging"
elif [ -d "/opt/vel" ] && [ -f "/opt/vel/bin/vel" ]; then
    VEL_DIR="/opt/vel"
    ENV="production"
else
    echo "No Vel installation found"
    exit 1
fi

cd "$VEL_DIR"

OUTPUT=$(./bin/vel verify 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
    FAILURES=$(echo "$OUTPUT" | grep -E "failed|FAIL" | tail -5)
    /home/claw/.npm-global/bin/openclaw wake "Vel verify FAILED on $ENV. Exit code: $EXIT_CODE. Failures: $FAILURES" 2>/dev/null
fi
