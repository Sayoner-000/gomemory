#!/usr/bin/env python3
"""Regresión de cancelación: instalador real bloqueado en seed y un descendiente escritor."""
import argparse
import errno
import fcntl
import json
import os
import pathlib
import pty
import select
import shutil
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time


GLOBAL_FIXTURE = r'''
import os, pathlib, subprocess, sys, time
root = pathlib.Path(os.environ["CANCEL_FIXTURE"])
if len(sys.argv) > 1 and sys.argv[1] == "seed":
    (root / "engine.pid").write_text(str(os.getppid()))
    (root / "seed.pid").write_text(str(os.getpid()))
    worker = subprocess.Popen([sys.executable, str(root / "writer.py")],
                              stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    (root / "writer.pid").write_text(str(worker.pid))
    while not (root / "heartbeat").exists():
        time.sleep(0.01)
    (root / "ready").write_text("ready")
    if os.environ.get("CANCEL_BAD_EVENT"):
        print('\x1egomemory-event:{invalid}', file=sys.stderr, flush=True)
    time.sleep(2)
elif len(sys.argv) > 1 and sys.argv[1] in ("version", "--version"):
    print("gomemory 2.29.0")
'''

WRITER_FIXTURE = r'''
import os, pathlib, time
heartbeat = pathlib.Path(os.environ["CANCEL_FIXTURE"]) / "heartbeat"
until = time.monotonic() + 15
i = 0
while time.monotonic() < until:
    i += 1
    heartbeat.write_text(str(i))
    time.sleep(0.03)
'''


def drain_terminal(master):
    if master is None:
        return b""
    ready, _, _ = select.select([master], [], [], 0)
    if not ready:
        return b""
    try:
        return os.read(master, 65536)
    except OSError as error:
        if error.errno == errno.EIO:
            return b""
        raise


def wait_for_file(path, process, master=None):
    deadline = time.monotonic() + 15
    terminal = bytearray()
    while time.monotonic() < deadline:
        terminal.extend(drain_terminal(master))
        if path.exists():
            return
        if process.poll() is not None:
            raise AssertionError(f"proceso terminó antes de seed: {process.returncode}")
        time.sleep(0.02)
    raise AssertionError(f"seed no inició: {terminal.decode(errors='replace')}")


def wait_process(process, master):
    deadline = time.monotonic() + 8
    while process.poll() is None:
        drain_terminal(master)
        if time.monotonic() > deadline:
            raise AssertionError("cancelación no terminó en 8 segundos")
        time.sleep(0.01)
    process.wait()


def run_case(binary, installer, lane, cancel_signal):
    with tempfile.TemporaryDirectory(prefix="gomemory-cancel-") as directory:
        root = pathlib.Path(directory)
        project, home, bin_dir = root / "project", root / "home", root / "bin"
        for path in (project, home, bin_dir):
            path.mkdir()
        (root / "writer.py").write_text(WRITER_FIXTURE)
        fake = bin_dir / "mem"
        fake.write_text(f"#!{sys.executable}\n" + GLOBAL_FIXTURE)
        fake.chmod(0o755)
        env = dict(os.environ, HOME=str(home), XDG_DATA_HOME=str(root / "data"),
                   XDG_CONFIG_HOME=str(root / "config"), PATH=str(bin_dir)+":/usr/bin:/bin",
                   CANCEL_FIXTURE=str(root), CI="", TERM="xterm-256color", GOMEMORY_NO_MOTION="1")
        for key in ("NO_COLOR", "GOMEMORY_INSTALL_EVENT_CHILD"):
            env.pop(key, None)
        if lane == "protocol":
            env["CANCEL_BAD_EVENT"] = "1"
        command = [str(binary), "install", str(project), "--agents", "none", "--scope", "project"]
        if lane != "native":
            command.append("--events")
        if lane == "typescript":
            command = [shutil.which("node"), str(installer), str(project), "--binary", str(binary),
                       "--yes", "--no-motion", "--agents", "none"]
        master = slave = None
        output = root / "stdout"
        error = root / "stderr"
        with output.open("wb") as stdout, error.open("wb") as stderr:
            if lane == "native":
                master, slave = pty.openpty()
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 100, 0, 0))
            process = subprocess.Popen(command, cwd=project, env=env, start_new_session=True,
                                       stdin=slave if slave is not None else subprocess.DEVNULL,
                                       stdout=slave if slave is not None else stdout, stderr=stderr)
            if slave is not None:
                os.close(slave)
            try:
                wait_for_file(root / "ready", process, master)
                if lane != "protocol":
                    # Señal solo al padre, no al grupo: no encubre al nieto huérfano.
                    process.send_signal(cancel_signal)
                wait_process(process, master)
                assert process.returncode != 0, (lane, "cancelación anunció éxito")
                before = (root / "heartbeat").read_text()
                time.sleep(0.8)
                after = (root / "heartbeat").read_text()
                assert before == after, (lane, "descendiente sigue escribiendo tras cierre", before, after)
                assert not (project / ".gitignore").exists(), (lane, "instalador continuó después de cancelar")
                if lane in ("native", "typescript"):
                    assert process.returncode == 130, (lane, process.returncode)
                if lane == "events":
                    events = [json.loads(line) for line in output.read_text().splitlines()]
                    assert events[-1]["status"] == "fail" and events[-1]["exit_code"] == 130
                print(f"✓ {lane} · {signal.Signals(cancel_signal).name} · descendientes detenidos antes del cierre")
            finally:
                # Limpiar también el árbol defectuoso durante la fase roja.
                for marker in ("writer.pid", "seed.pid", "engine.pid"):
                    if (root / marker).exists():
                        pid = int((root / marker).read_text())
                        try:
                            os.kill(pid, signal.SIGKILL)
                        except ProcessLookupError:
                            pass
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                if process.poll() is None:
                    process.kill()
                process.wait()
                if master is not None:
                    os.close(master)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("binary")
    parser.add_argument("--installer")
    parser.add_argument("--lane", choices=("native", "events", "typescript", "protocol"))
    args = parser.parse_args()
    binary = pathlib.Path(args.binary).resolve()
    installer = pathlib.Path(args.installer).resolve() if args.installer else None
    lanes = [args.lane] if args.lane else ["native", "events", "protocol"] + (["typescript"] if installer else [])
    for lane in lanes:
        if lane == "typescript" and installer is None:
            parser.error("typescript necesita --installer")
        for sig in ([signal.SIGTERM] if lane == "protocol" else [signal.SIGINT, signal.SIGTERM]):
            run_case(binary, installer, lane, sig)


if __name__ == "__main__":
    main()
