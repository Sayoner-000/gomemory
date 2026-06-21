#!/usr/bin/env bash
set -e

PORT=${GOMMY_PORT:-9735}

# Get previous session context
CONTEXT=$(curl -sf http://127.0.0.1:$PORT/context 2>/dev/null || echo "")

# Inject recovery instructions + previous context
cat << 'INSTRUCTIONS'
**AFTER COMPACTION — FIRST ACTION REQUIRED**

1. Call end_session() with a summary containing what we were working on,
   what was accomplished, and next steps.
2. Call get_context() to recover previous session state.
3. Only THEN continue working.

Do not skip step 1. Without it, everything done before compaction
is lost from memory.
INSTRUCTIONS

if [ -n "$CONTEXT" ]; then
  echo ""
  echo "Previous session context:"
  echo "$CONTEXT"
fi
