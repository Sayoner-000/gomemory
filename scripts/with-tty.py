#!/usr/bin/env python3
"""Ejecuta un comando con un TTY controlador y devuelve su código de salida.

Uso: python3 -I scripts/with-tty.py <comando> [args...]

Reproduce lo que ve una persona en su terminal aunque el llamador no tenga
TTY (CI, `mem hcd verify`). No lee la entrada estándar: pty.spawn sí lo hace
y, con una tubería que nadie cierra, se queda esperando tras el fin del hijo.
"""
import os
import pty
import sys

if len(sys.argv) < 2:
    sys.exit("uso: with-tty.py <comando> [args...]")

pid, fd = pty.fork()
if pid == 0:
    try:
        os.execvp(sys.argv[1], sys.argv[1:])
    finally:
        os._exit(127)

out = sys.stdout.buffer
while True:
    try:
        data = os.read(fd, 65536)
    except OSError:  # EIO: el hijo cerró el terminal.
        break
    if not data:
        break
    out.write(data.replace(b"\r\n", b"\n"))
    out.flush()

_, status = os.waitpid(pid, 0)
sys.exit(os.waitstatus_to_exitcode(status))
