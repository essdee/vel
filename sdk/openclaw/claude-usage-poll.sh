#!/usr/bin/env bash
# claude-usage-poll.sh — Fetch Claude Max 5hr/weekly usage via OAuth API
# Designed to run via cron (e.g., every 5 min) and drop JSON into the workspace.
# Includes exponential backoff on rate-limit responses (self-healing).
#
# Requirements:
#   - Claude Code CLI authenticated via `claude login` (NOT setup-token)
#   - jq installed
#   - Credentials at ~/.claude/.credentials.json (Linux) or macOS Keychain
#
# Output: Writes to CLAUDE_USAGE_OUTPUT (default: workspace/claude-usage.json)

set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
CLAUDE_USAGE_OUTPUT="${CLAUDE_USAGE_OUTPUT:-${OPENCLAW_WORKSPACE:-$HOME/.openclaw/workspace}/claude-usage.json}"
CREDENTIALS_FILE="${CLAUDE_CREDENTIALS_FILE:-$HOME/.claude/.credentials.json}"
OAUTH_CLIENT_ID="9d1c250a-e61b-44d9-88ed-5944d1962f5e"
USAGE_ENDPOINT="https://api.anthropic.com/api/oauth/usage"
TOKEN_ENDPOINT="https://api.anthropic.com/v1/oauth/token"
USER_AGENT="claude-code/2.1.34"
BETA_HEADER="oauth-2025-04-20"

# ---------------------------------------------------------------------------
# Backoff state
# ---------------------------------------------------------------------------
BACKOFF_FILE="${BACKOFF_FILE:-/tmp/claude-usage-backoff}"
BACKOFF_MIN=300       # 5 minutes initial backoff
BACKOFF_MAX=1800      # 30 minutes cap

check_backoff() {
  if [[ ! -f "$BACKOFF_FILE" ]]; then return 0; fi
  local until_ts
  until_ts=$(cat "$BACKOFF_FILE" 2>/dev/null | head -1)
  [[ -z "$until_ts" ]] && return 0
  local now_ts
  now_ts=$(date +%s)
  if (( now_ts < until_ts )); then
    local remaining=$(( until_ts - now_ts ))
    echo "[claude-usage] $(date '+%H:%M:%S') Backing off — ${remaining}s remaining" >&2
    exit 0
  fi
  # Backoff expired — clear it
  rm -f "$BACKOFF_FILE"
  return 0
}

set_backoff() {
  local now_ts
  now_ts=$(date +%s)
  # Double the previous backoff if exists, else start at BACKOFF_MIN
  local prev_duration=$BACKOFF_MIN
  if [[ -f "$BACKOFF_FILE" ]]; then
    local prev_until
    prev_until=$(sed -n '2p' "$BACKOFF_FILE" 2>/dev/null || echo "$BACKOFF_MIN")
    prev_duration=$(( prev_until * 2 ))
  fi
  if (( prev_duration > BACKOFF_MAX )); then
    prev_duration=$BACKOFF_MAX
  fi
  local until_ts=$(( now_ts + prev_duration ))
  printf '%s\n%s\n' "$until_ts" "$prev_duration" > "$BACKOFF_FILE"
  echo "[claude-usage] $(date '+%H:%M:%S') Rate limited — backing off ${prev_duration}s" >&2
}

# Check for on-demand trigger (written by token-swap server)
TRIGGER_FILE="${OPENCLAW_WORKSPACE:-$HOME/.openclaw/workspace}/.usage-poll-trigger"
if [[ -f "$TRIGGER_FILE" ]]; then
  rm -f "$TRIGGER_FILE"
  echo "[claude-usage] $(date '+%H:%M:%S') On-demand trigger detected — running immediately" >&2
  # Skip backoff check for triggered runs
else
  # Check backoff before doing anything (only for scheduled runs)
  check_backoff
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log() { echo "[claude-usage] $(date '+%H:%M:%S') $*" >&2; }

die() { log "ERROR: $*"; exit 1; }

read_credentials() {
  if [[ "$(uname)" == "Darwin" ]]; then
    # macOS: try Keychain first, fall back to file
    local kc_data
    kc_data=$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null || true)
    if [[ -n "$kc_data" ]]; then
      echo "$kc_data"
      return
    fi
  fi

  # Linux / fallback: read from file
  if [[ -f "$CREDENTIALS_FILE" ]]; then
    cat "$CREDENTIALS_FILE"
  else
    die "No credentials found. Run 'claude login' first (not setup-token)."
  fi
}

# ---------------------------------------------------------------------------
# Token management
# ---------------------------------------------------------------------------
get_access_token() {
  local creds_json
  creds_json=$(read_credentials)

  local access_token expires_at refresh_token scopes
  access_token=$(echo "$creds_json" | jq -r '.claudeAiOauth.accessToken // empty')
  refresh_token=$(echo "$creds_json" | jq -r '.claudeAiOauth.refreshToken // empty')
  expires_at=$(echo "$creds_json" | jq -r '.claudeAiOauth.expiresAt // 0')
  scopes=$(echo "$creds_json" | jq -r '.claudeAiOauth.scopes // [] | join(" ")')

  [[ -n "$access_token" ]] || die "No accessToken in credentials. Run 'claude login'."
  [[ -n "$refresh_token" ]] || die "No refreshToken in credentials. Run 'claude login'."

  # Check scopes — user:profile is required for /api/oauth/usage
  if [[ "$scopes" != *"user:profile"* ]]; then
    die "Token missing 'user:profile' scope. You authenticated with 'claude setup-token' instead of 'claude login'. Fix: delete credentials and run 'claude login' (full browser OAuth)."
  fi

  # Check expiry (expiresAt is ms epoch)
  local now_ms
  now_ms=$(date +%s)000
  # Add 5-minute buffer
  local buffer_ms=300000
  local threshold=$(( expires_at - buffer_ms ))

  if (( now_ms > threshold )); then
    log "Access token expired or expiring soon — refreshing..."
    refresh_access_token "$refresh_token" "$creds_json"
    return
  fi

  echo "$access_token"
}

refresh_access_token() {
  local refresh_token="$1"
  local old_creds="$2"

  local response
  response=$(curl -sS -X POST "$TOKEN_ENDPOINT" \
    -H "Content-Type: application/json" \
    -d "{
      \"grant_type\": \"refresh_token\",
      \"refresh_token\": \"$refresh_token\",
      \"client_id\": \"$OAUTH_CLIENT_ID\"
    }" 2>&1) || die "Token refresh request failed: $response"

  # Check for error
  local error_msg
  error_msg=$(echo "$response" | jq -r '.error.message // .error // empty' 2>/dev/null || true)
  if [[ -n "$error_msg" ]]; then
    die "Token refresh failed: $error_msg — Run 'claude login' to re-authenticate."
  fi

  local new_access new_refresh new_expires_in
  new_access=$(echo "$response" | jq -r '.access_token // empty')
  new_refresh=$(echo "$response" | jq -r '.refresh_token // empty')
  new_expires_in=$(echo "$response" | jq -r '.expires_in // 28800')

  [[ -n "$new_access" ]] || die "Refresh response missing access_token. Response: $response"

  # Calculate new expiresAt in ms
  local now_s
  now_s=$(date +%s)
  local new_expires_at=$(( (now_s + new_expires_in) * 1000 ))

  # Use the new refresh token if provided, otherwise keep the old one
  local final_refresh="${new_refresh:-$refresh_token}"

  # Update credentials file
  local updated_creds
  updated_creds=$(echo "$old_creds" | jq \
    --arg at "$new_access" \
    --arg rt "$final_refresh" \
    --argjson ea "$new_expires_at" \
    '.claudeAiOauth.accessToken = $at | .claudeAiOauth.refreshToken = $rt | .claudeAiOauth.expiresAt = $ea')

  # Save — file path
  if [[ -f "$CREDENTIALS_FILE" ]]; then
    echo "$updated_creds" > "$CREDENTIALS_FILE"
    chmod 600 "$CREDENTIALS_FILE"
    log "Credentials refreshed and saved to $CREDENTIALS_FILE"
  fi

  # macOS: also update Keychain
  if [[ "$(uname)" == "Darwin" ]]; then
    security delete-generic-password -s "Claude Code-credentials" 2>/dev/null || true
    security add-generic-password -s "Claude Code-credentials" -a "" -w "$updated_creds" 2>/dev/null || true
    log "Credentials refreshed and saved to Keychain"
  fi

  echo "$new_access"
}

# ---------------------------------------------------------------------------
# Fetch usage
# ---------------------------------------------------------------------------
fetch_usage() {
  local token="$1"

  local response
  response=$(curl -sS "$USAGE_ENDPOINT" \
    -H "Authorization: Bearer $token" \
    -H "anthropic-beta: $BETA_HEADER" \
    -H "User-Agent: $USER_AGENT" \
    -H "Accept: application/json" \
    -H "Content-Type: application/json" \
    2>&1) || die "Usage request failed: $response"

  # Check for rate limiting
  if echo "$response" | grep -qi "rate.limit"; then
    set_backoff
    exit 1
  fi

  # Check for errors
  local error_msg
  error_msg=$(echo "$response" | jq -r '.error.message // .error // empty' 2>/dev/null || true)
  if [[ -n "$error_msg" ]]; then
    # Also check if the error message mentions rate limiting
    if echo "$error_msg" | grep -qi "rate.limit"; then
      set_backoff
      exit 1
    fi
    die "Usage endpoint error: $error_msg"
  fi

  echo "$response"
}

# ---------------------------------------------------------------------------
# Format output
# ---------------------------------------------------------------------------
format_output() {
  local raw="$1"
  local now_iso
  now_iso=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

  # Build a clean output with human-readable summary
  jq --arg ts "$now_iso" '{
    fetched_at: $ts,
    five_hour: {
      utilization_pct: (.five_hour.utilization // null),
      resets_at: (.five_hour.resets_at // null)
    },
    seven_day: {
      utilization_pct: (.seven_day.utilization // null),
      resets_at: (.seven_day.resets_at // null)
    },
    seven_day_opus: {
      utilization_pct: (.seven_day_opus.utilization // null),
      resets_at: (.seven_day_opus.resets_at // null)
    },
    seven_day_sonnet: {
      utilization_pct: (.seven_day_sonnet.utilization // null),
      resets_at: (.seven_day_sonnet.resets_at // null)
    },
    summary: (
      "5hr: " + ((.five_hour.utilization // 0) | tostring) + "% | " +
      "7day: " + ((.seven_day.utilization // 0) | tostring) + "% | " +
      "Opus: " + ((.seven_day_opus.utilization // 0) | tostring) + "% | " +
      "Sonnet: " + ((.seven_day_sonnet.utilization // 0) | tostring) + "%"
    )
  }' <<< "$raw"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  # Ensure output directory exists
  mkdir -p "$(dirname "$CLAUDE_USAGE_OUTPUT")"

  local token
  token=$(get_access_token)

  local raw_usage
  raw_usage=$(fetch_usage "$token")

  local formatted
  formatted=$(format_output "$raw_usage")

  # Write atomically
  local tmp="${CLAUDE_USAGE_OUTPUT}.tmp"
  echo "$formatted" > "$tmp"
  mv "$tmp" "$CLAUDE_USAGE_OUTPUT"

  # Clear backoff on success
  rm -f "$BACKOFF_FILE"

  local summary
  summary=$(echo "$formatted" | jq -r '.summary')
  log "OK — $summary → $CLAUDE_USAGE_OUTPUT"
}

main "$@"
