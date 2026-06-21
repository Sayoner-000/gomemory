#!/usr/bin/env bash
set -e

PORT=${GOMMY_PORT:-9735}

# Close active session
if [ -n "$1" ]; then
  curl -sf -X POST http://127.0.0.1:$PORT/session/end \
    -H "Content-Type: application/json" \
    -d "{\"session_id\": \"$1\", \"summary\": \"$2\"}" >/dev/null 2>&1 || true
else
  curl -sf -X POST http://127.0.0.1:$PORT/session/end \
    -H "Content-Type: application/json" \
    -d '{"summary": "session ended"}' >/dev/null 2>&1 || true
fi
