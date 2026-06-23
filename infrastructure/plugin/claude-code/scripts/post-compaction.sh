#!/usr/bin/env bash
# Wrapper fino y legado: delega en el binario portable (`mem hook pre-compact`).
exec {{BIN_PATH}} hook pre-compact "$@"
