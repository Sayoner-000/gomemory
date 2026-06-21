#!/usr/bin/env bash
set -e

PORT=${GOMMY_PORT:-9735}
# First prompt: inject ToolSearch so MCP tools are loaded
if [ ! -f /tmp/gomemory-first-prompt-done ]; then
  echo '{"tools": true}'
  touch /tmp/gomemory-first-prompt-done
else
  # Subsequent prompts: passive, no overhead
  echo "{}"
fi
