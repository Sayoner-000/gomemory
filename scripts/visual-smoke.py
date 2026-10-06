#!/usr/bin/env python3
"""Comprueba el binario servido por PTY y la instalación real en un HOME aislado."""
import argparse
import errno
import fcntl
import json
import os
import pathlib
import pty
import re
import select
import shutil
import struct
import subprocess
import tempfile
import termios
import time
import unicodedata

ANSI = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")


def visible_width(text):
    return sum(0 if unicodedata.combining(c) else 2 if unicodedata.east_asian_width(c) in "WF" else 1 for c in text)


def terminal(command, directory, environment, columns, answers=()):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, columns, 0, 0))
    process = subprocess.Popen(command, cwd=directory, env=environment, stdin=slave, stdout=slave, stderr=subprocess.PIPE)
    os.close(slave)
    data = bytearray()
    pending_answers = list(answers)
    deadline = time.monotonic() + 30
    try:
        while time.monotonic() < deadline:
            ready, _, _ = select.select([master], [], [], 0.1)
            if ready:
                try:
                    chunk = os.read(master, 65536)
                except OSError as error:
                    if error.errno == errno.EIO:
                        break
                    raise
                if not chunk:
                    break
                data.extend(chunk)
                if pending_answers and pending_answers[0][0].encode() in data:
                    _, keys = pending_answers.pop(0)
                    os.write(master, keys)
            elif process.poll() is not None:
                break
        else:
            raise AssertionError(f"terminal bloqueada: {command}")
        process.wait(timeout=5)
        assert process.returncode == 0, (command, process.stderr.read().decode())
        return data.decode("utf-8").replace("\r\n", "\n")
    finally:
        if process.poll() is None:
            process.kill()
            process.wait()
        os.close(master)
        process.stderr.close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("binary")
    parser.add_argument("--installer", help="Ruta al CLI TypeScript compilado")
    parser.add_argument("--preview", action="store_true")
    args = parser.parse_args()
    binary = pathlib.Path(args.binary).resolve()
    with tempfile.TemporaryDirectory(prefix="gomemory-visual-") as directory:
        root = pathlib.Path(directory)
        project = root / "mi proyecto"
        home = root / "home"
        bin_dir = root / "bin"
        for path in (project, home, bin_dir):
            path.mkdir()
        local_binary = bin_dir / "mem"
        shutil.copy2(binary, local_binary)
        env = dict(os.environ, HOME=str(home), XDG_DATA_HOME=str(root / "data"), XDG_CONFIG_HOME=str(root / "config"),
                   PATH=str(bin_dir)+":/usr/bin:/bin", TERM="xterm-256color", GOMEMORY_NO_MOTION="1", CI="")
        for name in ("NO_COLOR", "FORCE_COLOR", "GOMEMORY_INSTALL_EVENT_CHILD"):
            env.pop(name, None)
        subprocess.run([str(local_binary), "save", "-t", "Decisión con acentos", "-y", "decision", "Aprendizaje con símbolos 界"], cwd=project, env=env, check=True, capture_output=True)
        for theme in ("dark", "light", "matrix"):
            env["GOMEMORY_THEME"] = theme
            for width in (40, 80, 120):
                for command in (["help"], ["help", "install"], ["list"], ["search", "desconocido"], ["search", "json"], ["usage"], ["settings", "--show"], ["doctor"]):
                    result = terminal([str(local_binary), *command], project, env, width)
                    plain = ANSI.sub("", result)
                    if command[0] == "search":
                        assert "goMemory › search" in plain, (command, plain)
                    for line in plain.splitlines():
                        assert visible_width(line) <= width, (theme, width, command, visible_width(line), line)
                    if args.preview and theme == "dark" and width == 80 and command == ["help"]:
                        print(result)
                encoded = terminal([str(local_binary), "usage", "--json"], project, env, width)
                assert "\x1b" not in encoded
                json.loads(encoded)
                print(f"✓ PTY {theme} {width} columnas · help/list/search/usage/settings/doctor + JSON")
        env["CI"] = "1"

        native_env = dict(env, CI="", PATH=str(bin_dir), GOMEMORY_THEME="dark")
        assert shutil.which("node", path=native_env["PATH"]) is None
        assert shutil.which("npm", path=native_env["PATH"]) is None
        native = terminal([str(local_binary), "install", str(project), "--agents", "none", "--scope", "project"], project, native_env, 80)
        native_plain = ANSI.sub("", native)
        for expected in ("┌", "│", "✓ Memoria", "└", "goMemory listo"):
            assert expected in native_plain, (expected, native_plain)
        assert '"contract_version"' not in native_plain
        assert "gomemory instalado. Ahora puedes" not in native_plain
        for line in native_plain.splitlines():
            assert visible_width(line) <= 80, line
        print("✓ mem install nativo en PTY · PATH sin Node/npm · recorrido visual en el binario")

        cancel_project = root / "cancelar"
        cancel_project.mkdir()
        canceled = terminal([str(local_binary), "install", str(cancel_project)], cancel_project, native_env, 80,
                            [("¿Qué agentes", b"\x1b")])
        assert "Cancelado" in ANSI.sub("", canceled)
        assert list(cancel_project.iterdir()) == []
        interactive = terminal([str(local_binary), "install", str(project)], project, native_env, 80,
                               [("¿Qué agentes", b"\r"), ("¿Alcance", b"\r"), ("¿Continuar?", b"\r")])
        assert "goMemory listo" in ANSI.sub("", interactive)
        assert "◇" in ANSI.sub("", interactive)
        print("✓ Preguntas reales con teclado · confirmación · Esc cancela sin escribir")

        global_project = root / "global-project"
        global_project.mkdir()
        global_directory = root / ("global-" + "g" * 90)
        global_env = dict(native_env, PATH="/usr/bin:/bin", GOMEMORY_BIN_DIR=str(global_directory))
        assert shutil.which("mem", path=global_env["PATH"]) is None
        global_output = terminal([str(local_binary), "install", str(global_project), "--agents", "none", "--scope", "project"], global_project, global_env, 80)
        global_plain = ANSI.sub("", global_output)
        assert "Binario global" in global_plain
        for line in global_plain.splitlines():
            assert visible_width(line) <= 80, ("instalación global", line)
        print("✓ Instalación global con ruta larga · mensajes dentro del ancho de terminal")

        install = subprocess.run([str(local_binary), "install", str(project), "--events", "--agents", "none"], env=env, capture_output=True, text=True, timeout=45)
        assert install.returncode == 0, install.stderr
        events = [json.loads(line) for line in install.stdout.splitlines()]
        assert events[0]["type"] == "start" and events[-1]["type"] == "complete"
        assert any(e.get("name") == "Memoria" for e in events)
        assert events[-1]["status"] in ("ok", "warn")
        failed = subprocess.run([str(local_binary), "install", str(root / "ausente"), "--events"], env=env, capture_output=True, text=True, timeout=30)
        assert failed.returncode != 0 and json.loads(failed.stdout.splitlines()[-1])["status"] == "fail"
        print("✓ Instalación nativa real NDJSON · sin agentes · error de destino")
        if args.installer:
            node = shutil.which("node")
            install = subprocess.run([node, str(pathlib.Path(args.installer).resolve()), str(project), "--yes", "--no-motion", "--agents", "none", "--binary", str(local_binary)], env=env, capture_output=True, text=True, timeout=45)
            assert install.returncode == 0, install.stdout+install.stderr
            assert "goMemory listo" in install.stdout or "avisos" in install.stdout
            assert "\x1b" not in install.stdout+install.stderr
            print("✓ Instalador TypeScript real → motor Go · reinstalación sin prompts")


if __name__ == "__main__":
    main()
