#!/usr/bin/env bash
# Wrapper fino y legado: delega en el binario portable (`mem hook user-prompt-submit`).
exec {{BIN_PATH}} hook user-prompt-submit "$@"
