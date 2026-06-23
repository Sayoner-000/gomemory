#!/usr/bin/env bash
# Wrapper fino y legado: delega en el binario portable (`mem hook session-end`).
exec {{BIN_PATH}} hook session-end "$@"
