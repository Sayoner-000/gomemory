#!/usr/bin/env bash
set -e

PORT=${GOMMY_PORT:-9735}
BIN="{{BIN_PATH}}"

# Ensure HTTP server is running
if ! curl -sf http://127.0.0.1:$PORT/health >/dev/null 2>&1; then
  $BIN serve --port $PORT &
  sleep 1
fi

# Create session
SESSION=$(curl -sf -X POST http://127.0.0.1:$PORT/session/start)
echo "$SESSION"

# Import git-synced memories if manifest exists
if [ -f ".memory/manifest.json" ]; then
  $BIN sync --import 2>/dev/null || true
fi

# Inject context from previous sessions
CONTEXT=$(curl -sf http://127.0.0.1:$PORT/context)
if [ -n "$CONTEXT" ]; then
  echo "Previous context loaded: $CONTEXT"
fi
