#!/usr/bin/env bash
# Wrapper fino y legado: delega en el binario portable.
# El camino soportado es `mem hook session-start` (ver hooks.json / settings.json).
# {{BIN_PATH}} se reemplaza en instalación; por defecto es `mem` (en el PATH).
exec {{BIN_PATH}} hook session-start "$@"
