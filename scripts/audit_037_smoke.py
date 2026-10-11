#!/usr/bin/env python3
"""Runner de validación de la feature 037 (cierre de auditoría defensiva).

Ejecuta casos contra el binario real de `mem` en un banco aislado: HOME,
XDG_* y GOMEMORY_DATA_HOME temporales para cada proceso hijo. Nunca toca el
almacén ni la configuración habituales del operador.

Códigos de salida (quickstart.md, "Contrato de runners previstos"):
  0  todos los casos seleccionados se ejecutaron y aprobaron
  1  al menos un caso reproduce un defecto o falla
  2  preparación inválida, runtime incompatible o cero casos ejecutados

Solo biblioteca estándar.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import sqlite3
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

MIN_PYTHON = (3, 12)
SAFE_SYSTEM_PATH = "/usr/bin:/bin:/usr/sbin:/sbin"

EXIT_OK = 0
EXIT_DEFECT = 1
EXIT_PREPARATION = 2


class PreparationError(Exception):
    """Falta un prerrequisito: el resultado no es evidencia válida."""


@dataclass
class Bench:
    mem_bin: Path
    work_dir: Path
    data_home: Path
    out_dir: Path
    env: dict[str, str] = field(default_factory=dict)


@dataclass
class CaseResult:
    case_id: str
    ok: bool
    detail: str = ""


def preflight() -> None:
    if sys.version_info < MIN_PYTHON:
        found = ".".join(str(p) for p in sys.version_info[:3])
        need = ".".join(str(p) for p in MIN_PYTHON)
        raise PreparationError(f"RUNTIME_UNSUPPORTED: Python {found} < {need}")


def isolated_env(root: Path, data_home: Path) -> dict[str, str]:
    """Entorno hijo con todas las rutas de usuario dentro del banco."""
    env = {k: v for k, v in os.environ.items() if not k.startswith(("XDG_", "GOMEMORY_"))}
    home = root / "home"
    for sub in ("home", "config", "data", "state", "cache"):
        (root / sub).mkdir(parents=True, exist_ok=True)
    env.update(
        # PATH mínimo del sistema: un `mem` del usuario en el PATH heredado
        # sería un destino real para comandos que resuelven el binario global
        # (mem update lo reemplazó en una ejecución anterior de este runner).
        PATH=SAFE_SYSTEM_PATH,
        HOME=str(home),
        XDG_CONFIG_HOME=str(root / "config"),
        XDG_DATA_HOME=str(root / "data"),
        XDG_STATE_HOME=str(root / "state"),
        XDG_CACHE_HOME=str(root / "cache"),
        GOMEMORY_DATA_HOME=str(data_home),
        GOMEMORY_NO_UPDATE_CHECK="1",
    )
    return env


# Marcadores sintéticos: ninguno es un secreto real. La credencial es la clave
# de ejemplo de la documentación de AWS, reconocida por la política vigente.
PRIV = "SECRETO_037"
CRED = "AKIAIOSFODNN7EXAMPLE"
PUBLIC = "texto publico util"
PROTECTED = (
    f"{PUBLIC} <private>{PRIV} <PRIVATE>{PRIV}_ANIDADO</private> ü 秘密</Private> "
    f"clave {CRED} <private data-motivo='x' >{PRIV}_VARIANTE</private > "
    f"<private>{PRIV}_ABIERTO hasta el final"
)
# La evidencia nombra la categoría, nunca el marcador literal.
MARKERS = {PRIV: "privado", CRED: "credencial"}


@dataclass
class Run:
    code: int
    out: str
    err: str


def mem(bench: Bench, *args: str, stdin: str | None = None, cwd: Path | None = None, timeout: int = 60) -> Run:
    proc = subprocess.run(
        [str(bench.mem_bin), *args],
        cwd=cwd or bench.work_dir,
        env=bench.env,
        input=stdin if stdin is not None else "",
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    return Run(proc.returncode, proc.stdout, proc.stderr)


def as_json(run: Run) -> dict:
    """Objeto JSON de stdout, o {} si la salida no lo es (p. ej. comando
    inexistente en una revisión anterior): el caso falla, no el runner."""
    try:
        data = json.loads(run.out)
    except ValueError:
        return {}
    return data if isinstance(data, dict) else {}


def leaks(text: str) -> list[str]:
    return [label for m, label in MARKERS.items() if m in text]


def check(results: list[CaseResult], case_id: str, ok: bool, detail: str = "") -> bool:
    results.append(CaseResult(case_id, ok, "" if ok else detail))
    return ok


def check_output(results: list[CaseResult], case_id: str, run: Run, need_public: str | None = None) -> None:
    found = leaks(run.out + run.err)
    if run.code != 0:
        check(results, case_id, False, f"salida {run.code}: {run.err.strip()[:200]}")
    elif found:
        check(results, case_id, False, f"revela {found}")
    elif need_public and need_public not in run.out:
        check(results, case_id, False, f"perdió el texto público {need_public!r}")
    else:
        check(results, case_id, True)


def project_db(bench: Bench, project: Path | None = None) -> Path:
    """Base del proyecto del caso dentro del almacén aislado del banco."""
    name = (project or bench.work_dir).name
    match = [d for d in bench.data_home.glob("projects/*/mem.db") if d.parent.name.startswith(name + "-")]
    if len(match) != 1:
        raise PreparationError(f"base del proyecto {name} no localizada")
    return match[0]


def store_bytes_leaks(bench: Bench) -> list[str]:
    """Busca marcadores en todos los archivos del almacén y del .memory."""
    found = []
    roots = [bench.data_home, bench.work_dir / ".memory"]
    for root in roots:
        if not root.exists():
            continue
        for path in root.rglob("*"):
            if not path.is_file():
                continue
            data = path.read_bytes()
            for m, label in MARKERS.items():
                if m.encode() in data:
                    found.append(f"{path.relative_to(root)}:{label}")
    return found


class MCPClient:
    """Cliente mínimo de `mem mcp` por stdio (JSON-RPC por líneas)."""

    def __init__(self, bench: Bench):
        self.proc = subprocess.Popen(
            [str(bench.mem_bin), "mcp"], cwd=bench.work_dir, env=bench.env,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
        )
        self.next_id = 0
        self.request("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                    "clientInfo": {"name": "audit-037", "version": "1"}})
        self.notify("notifications/initialized")

    def notify(self, method: str, params: dict | None = None) -> None:
        self._send({"jsonrpc": "2.0", "method": method, **({"params": params} if params else {})})

    def _send(self, msg: dict) -> None:
        assert self.proc.stdin is not None
        self.proc.stdin.write(json.dumps(msg) + "\n")
        self.proc.stdin.flush()

    def request(self, method: str, params: dict) -> dict:
        self.next_id += 1
        rid = self.next_id
        self._send({"jsonrpc": "2.0", "id": rid, "method": method, "params": params})
        assert self.proc.stdout is not None
        while True:
            line = self.proc.stdout.readline()
            if not line:
                raise PreparationError("mem mcp cerró la conexión")
            msg = json.loads(line)
            if msg.get("id") == rid:
                return msg

    def call_many(self, tool: str, args: dict, n: int) -> list[dict]:
        """Envía n llamadas sin esperar respuesta (solapadas en la misma
        conexión) y devuelve las n respuestas."""
        ids = []
        for _ in range(n):
            self.next_id += 1
            ids.append(self.next_id)
            self._send({"jsonrpc": "2.0", "id": self.next_id, "method": "tools/call",
                        "params": {"name": tool, "arguments": args}})
        assert self.proc.stdout is not None
        got: dict[int, dict] = {}
        while len(got) < n:
            line = self.proc.stdout.readline()
            if not line:
                raise PreparationError("mem mcp cerró la conexión")
            msg = json.loads(line)
            if msg.get("id") in ids:
                got[msg["id"]] = msg
        return [got[i] for i in ids]

    def call(self, tool: str, args: dict) -> str:
        msg = self.request("tools/call", {"name": tool, "arguments": args})
        return json.dumps(msg, ensure_ascii=False)

    def close(self) -> str:
        assert self.proc.stdin is not None
        self.proc.stdin.close()
        try:
            _, err = self.proc.communicate(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            _, err = self.proc.communicate()
        return err


def case_privacy(bench: Bench) -> list[CaseResult]:
    """US1 / H-01 nuevas escrituras y H-02: todas las entradas reales."""
    r: list[CaseResult] = []

    run = mem(bench, "save", "-t", "titulo " + PROTECTED, "-y", "decision", PROTECTED)
    check(r, "privacy.save", run.code == 0 and not leaks(run.out + run.err), run.err[:200])

    run = mem(bench, "session", "start")
    if run.code != 0:
        raise PreparationError(f"session start: {run.err[:200]}")
    check_output(r, "privacy.session_end_cli", mem(bench, "session", "end", "-s", PROTECTED), PUBLIC)

    mcp = MCPClient(bench)
    try:
        mcp.call("start_session", {})
        out = mcp.call("save_session_summary", {"summary": PROTECTED})
        check(r, "privacy.mcp_save_session_summary", not leaks(out), f"respuesta revela {leaks(out)}")
        out = mcp.call("end_session", {"summary": PROTECTED})
        check(r, "privacy.mcp_end_session", not leaks(out), f"respuesta revela {leaks(out)}")
    finally:
        err = mcp.close()
    check(r, "privacy.mcp_stderr", not leaks(err), f"stderr revela {leaks(err)}")

    mem(bench, "session", "start")
    run = mem(bench, "hook", "compact-summary", stdin=json.dumps({"compact_summary": PROTECTED}))
    check(r, "privacy.hook_compact_summary", run.code == 0 and not leaks(run.out + run.err), run.err[:200])
    run = mem(bench, "hook", "channel-error", "opencode", "user", "turn_reminder", "fallo " + PROTECTED)
    check(r, "privacy.hook_channel_error", run.code == 0 and not leaks(run.out + run.err), run.err[:200])

    mem(bench, "save", "-t", "otra", "-y", "decision", "otra memoria publica")
    db = project_db(bench)
    with sqlite3.connect(db) as con:
        ids = [row[0] for row in con.execute("SELECT id FROM memories ORDER BY id")]
    if len(ids) < 2:
        raise PreparationError("no se crearon las dos memorias para relacionar")
    check_output(r, "privacy.relation_create", mem(bench, "compare", "-r", "related", "-c", "0.7", "-m", PROTECTED, str(ids[0]), str(ids[1])))
    check_output(r, "privacy.relation_update", mem(bench, "compare", "-r", "supersedes", "-c", "0.8", "-m", PROTECTED, str(ids[0]), str(ids[1])))
    check_output(r, "privacy.relation_list", mem(bench, "compare", "list"))
    mcp = MCPClient(bench)
    try:
        out = mcp.call("judge_memories", {"id_a": ids[0], "id_b": ids[1], "verdict": "related",
                                          "confidence": 0.6, "reasoning": PROTECTED})
        check(r, "privacy.mcp_judge_memories", '"isError":true' not in out.replace(" ", "") and not leaks(out),
              f"respuesta revela {leaks(out)} o error")
    finally:
        mcp.close()

    export_path = bench.out_dir / "export.json"
    bench.out_dir.mkdir(parents=True, exist_ok=True)
    run = mem(bench, "export", "--out", str(export_path))
    exported = export_path.read_text() if export_path.exists() else ""
    check(r, "privacy.export", run.code == 0 and exported != "" and not leaks(exported),
          f"salida {run.code} o export revela {leaks(exported)}")

    bundle = json.loads(exported) if exported else {"memories": []}
    bundle["relations"] = [{"ref_a": m["ref_id"], "ref_b": n["ref_id"], "relation": "related",
                            "confidence": 0.4, "reasoning": PROTECTED, "created_at": "2019-09-09 09:09:09"}
                           for m, n in zip(bundle["memories"][:1], bundle["memories"][1:2])]
    other = bench.work_dir.parent / (bench.work_dir.name + "-import")
    prepare_project(other)
    bundle_path = bench.out_dir / "bundle-protegido.json"
    bundle_path.write_text(json.dumps(bundle))
    run = mem(bench, "import", str(bundle_path), cwd=other)
    check(r, "privacy.import", run.code == 0 and not leaks(run.out + run.err), run.err[:200])
    bundle_path.unlink()

    check_output(r, "privacy.context", mem(bench, "context"), PUBLIC)
    check_output(r, "privacy.session_list", mem(bench, "session", "list"))

    big_public = "".join(f"2026-10-11T10:00:{i % 60:02d} INFO worker-{i % 7} batch {i} ok\n" for i in range(400))
    pub_path = bench.out_dir / "publico.log"
    pub_path.write_text(big_public)
    run = mem(bench, "pack", "compress", "--level", "max", "--json", str(pub_path))
    refs = as_json(run).get("refs") or []
    if check(r, "privacy.original_public_stored", len(refs) == 1, f"salida {run.code} refs={refs}"):
        got = mem(bench, "pack", "retrieve", refs[0])
        check(r, "privacy.original_public_exact", got.code == 0 and got.out.rstrip("\n") == big_public.rstrip("\n"),
              f"salida {got.code}, recuperación no exacta")
    priv_path = bench.out_dir / "privado.log"
    priv_path.write_text(big_public + f"<private>{PRIV} sin cierre\n" + big_public)
    run = mem(bench, "pack", "compress", "--level", "max", "--json", str(priv_path))
    data = as_json(run)
    check(r, "privacy.original_private_not_stored", run.code == 0 and not data.get("refs"),
          f"salida {run.code} refs={data.get('refs')}")
    priv_path.unlink()

    found = store_bytes_leaks(bench)
    check(r, "privacy.store_bytes", not found, f"marcadores en almacén: {found[:5]}")
    return r


def prepare_project(project: Path) -> None:
    """Proyecto git aislado, sin grafo externo: su refresco en segundo plano
    no aporta a estos casos y no debe tocar nada fuera del banco."""
    project.mkdir(parents=True, exist_ok=True)
    if not (project / ".git").exists():
        subprocess.run(["git", "-C", str(project), "init", "-q"], check=True)
    settings = project / ".memory" / "settings.json"
    settings.parent.mkdir(exist_ok=True)
    if not settings.exists():
        settings.write_text('{"code_graph_disabled": true}\n')


def case_history(bench: Bench) -> list[CaseResult]:
    """US2 / H-01 histórico: saneamiento explícito, interrupciones y residuos."""
    import gzip
    import time

    r: list[CaseResult] = []
    if mem(bench, "session", "start").code != 0:
        raise PreparationError("session start falló")
    db = project_db(bench)
    key = db.parent.name
    protected = f"{PUBLIC} <private>{PRIV}</private> {CRED}"
    with sqlite3.connect(db) as con:
        con.execute("INSERT INTO sessions (id, project, summary, created_at, ended_at) VALUES ('hist-s', ?, ?, '2025-01-01 00:00:00', '2025-01-01 02:00:00')", (key, protected))
        con.execute("INSERT INTO memory_relations (id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at) VALUES (77, ?, 1, 2, 'related', 0.42, ?, '2024-04-04 04:04:04')", (key, protected))
        con.execute("INSERT INTO channel_activity (project, agent, scope, kind, last_error, last_error_at) VALUES (?, 'opencode', 'user', 'turn_reminder', ?, '2025-03-03 00:00:00')", (key, protected))
        con.execute("INSERT INTO compression_originals (ref, hash, project, content_gz, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at) VALUES ('privref00001', 'hh', ?, ?, 10, 'log', 'log', 'x', 'x', '2099-01-01')",
                    (key, gzip.compress(f"salida <private>{PRIV} sin cierre".encode())))
        con.execute("INSERT INTO memories (id, project, type, title, content) VALUES (900, ?, 'decision', 'remanente', 'texto Z</private> fin')", (key,))
    backups = bench.data_home / "backups" / key
    backups.mkdir(parents=True, exist_ok=True)
    bundle = {"version": 2, "exported_at": "x", "source_project": key, "memories": [],
              "relations": [{"ref_a": 1, "ref_b": 2, "relation": "related", "confidence": 0.4,
                             "reasoning": protected, "created_at": "2024-01-01 00:00:00"}]}
    (backups / "20250101-000000.000000000.json").write_text(json.dumps(bundle))
    locked = backups / "20250102-000000.000000000.json"
    locked.write_text(json.dumps(bundle))
    context_md = bench.work_dir / ".memory" / "context.md"
    context_md.write_text(f"## Sesiones\n- {protected}\n")

    def snapshot() -> list:
        with sqlite3.connect(db) as con:
            return con.execute("SELECT id, summary, created_at, ended_at FROM sessions ORDER BY id").fetchall()

    before = snapshot()
    run = mem(bench, "sanitize", "--json")
    report = as_json(run)
    cats = {c["category"]: c["affected_count"] for c in report.get("categories", [])}
    check(r, "history.preview", run.code == 0 and not leaks(run.out + run.err) and snapshot() == before
          and cats.get("session") == 1 and cats.get("memory_review") == 1, f"salida {run.code} categorías {cats}")

    # Cancelación: un plazo mínimo no puede declarar éxito ni confirmar cambios.
    run = mem(bench, "sanitize", "--apply", "--json", "--timeout", "1ns")
    check(r, "history.cancel", run.code == 1 and snapshot() == before, f"salida {run.code}")

    # Escritor simultáneo: interrupción antes del commit.
    writer = sqlite3.connect(db, timeout=1, isolation_level=None)
    writer.execute("BEGIN IMMEDIATE")
    run = mem(bench, "sanitize", "--apply", "--json", "--timeout", "30s")
    writer.execute("ROLLBACK")
    writer.close()
    check(r, "history.concurrent_writer", run.code == 1 and "SANITIZE_BUSY" in run.err and snapshot() == before,
          f"salida {run.code} stderr {run.err.strip()[:120]}")

    # Copia inaccesible: queda pendiente, no se borra a ciegas.
    locked.chmod(0)
    run = mem(bench, "sanitize", "--apply", "--json", "--timeout", "30s")
    locked.chmod(0o600)
    pending = as_json(run).get("status", "")
    check(r, "history.unreadable_copy", run.code == 1 and pending == "pending_cleanup" and locked.exists(),
          f"salida {run.code} estado {pending}")

    # Lector que bloquea el checkpoint: interrupción después del commit.
    reader = sqlite3.connect(db, isolation_level=None)
    reader.execute("BEGIN")
    reader.execute("SELECT COUNT(*) FROM sessions").fetchall()
    run = mem(bench, "sanitize", "--apply", "--json", "--timeout", "30s")
    reader.execute("ROLLBACK")
    reader.close()
    status = as_json(run).get("status", "")
    check(r, "history.checkpoint_blocked", run.code == 1 and status == "pending_cleanup" and "SANITIZE_BUSY" in run.err,
          f"salida {run.code} estado {status}")

    time.sleep(0.2)
    run = mem(bench, "sanitize", "--apply", "--json")
    final = as_json(run)
    check(r, "history.retry_completes", run.code == 0 and final.get("status") == "completed",
          f"salida {run.code} estado {final.get('status')}")
    after = snapshot()
    same_identity = [(row[0], row[2], row[3]) for row in after] == [(row[0], row[2], row[3]) for row in before]
    check(r, "history.identity_preserved", same_identity, "ids o fechas alteradas")

    run = mem(bench, "sanitize", "--apply", "--json")
    again = as_json(run)
    check(r, "history.idempotent", run.code == 0 and again.get("rows_changed") == 0, f"salida {run.code} filas {again.get('rows_changed')}")

    reports = list((bench.data_home / "maintenance" / "privacy").glob("*.json"))
    statuses = {json.loads(p.read_text()).get("status") for p in reports}
    check(r, "history.reports_closed", reports != [] and statuses == {"completed"}, f"estados {statuses}")

    wal_files = [p.name for p in db.parent.iterdir()]
    found = store_bytes_leaks(bench)
    check(r, "history.residues", not found and "mem.db" in wal_files, f"residuos: {found[:5]}")
    return r


def case_context(bench: Bench) -> list[CaseResult]:
    """US3 / H-03: emisiones repetidas y solapadas con métricas idénticas."""
    r: list[CaseResult] = []
    settings = bench.work_dir / ".memory" / "settings.json"
    settings.write_text(json.dumps({"code_graph_disabled": True, "budget": 3000}))
    if mem(bench, "session", "start").code != 0:
        raise PreparationError("session start falló")
    db = project_db(bench)
    key = db.parent.name
    with sqlite3.connect(db) as con:
        for i in range(40):
            con.execute("INSERT INTO memories (project, type, title, content, created_at, updated_at) VALUES (?, 'decision', ?, ?, '2026-10-01 00:00:00', '2026-10-01 00:00:00')",
                        (key, f"decisión {i}", f"contenido {i} " * 30))

    outs = [mem(bench, "context") for _ in range(3)]
    check(r, "context.cli_sequential", all(o.code == 0 for o in outs) and outs[0].out == outs[1].out == outs[2].out,
          "las tres emisiones de mem context difieren")

    # Equivalente aislado: una única llamada en un proceso MCP nuevo.
    mcp = MCPClient(bench)
    try:
        mcp.call("get_context", {"full": True})
    finally:
        mcp.close()
    mcp = MCPClient(bench)
    try:
        replies = mcp.call_many("get_context", {"full": True}, 8)
    finally:
        mcp.close()
    texts = [json.dumps(m.get("result", {}).get("content", []), ensure_ascii=False) for m in replies]
    check(r, "context.mcp_overlapped_content", all("error" not in m for m in replies) and len(set(texts)) == 1,
          "respuestas MCP con error o contenido distinto")

    with sqlite3.connect(db) as con:
        rows = con.execute("SELECT channel, baseline_tokens, emitted_tokens FROM usage_records WHERE project = ? AND operation = 'build_context' ORDER BY id", (key,)).fetchall()
    mcp_rows = [(b, e) for ch, b, e in rows if ch == "mcp"]
    isolated, overlapped = mcp_rows[:1], mcp_rows[1:]
    check(r, "context.discards_present", bool(isolated) and isolated[0][0] > isolated[0][1], f"sin descartes: {isolated}")
    check(r, "context.mcp_metrics_isolated", len(overlapped) == 8 and len(set(overlapped)) == 1,
          f"emisiones MCP: {sorted(set(overlapped))} ({len(overlapped)} filas)")
    check(r, "context.mcp_equals_isolated", bool(isolated) and set(overlapped) == set(isolated),
          f"solapadas {sorted(set(overlapped))[:2]} vs aislada {isolated}")
    return r


REPO_ROOT = Path(__file__).resolve().parent.parent
FAKE_CURL_DIR = REPO_ROOT / "tests" / "testdata" / "install037" / "bin"
INSTALL_TOOLS = ("bash", "env", "uname", "mktemp", "rm", "tar", "gzip", "mkdir", "install", "cp", "chmod", "mv",
                 "cat", "sed", "grep", "awk", "head", "od", "tr", "tput", "dirname", "basename", "wc", "cut",
                 "sort", "uniq", "printf", "test", "ls", "stat")


def platform_asset() -> tuple[str, str, str]:
    import platform
    os_name = {"Darwin": "darwin", "Linux": "linux"}.get(platform.system())
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine())
    if not os_name or not arch:
        raise PreparationError(f"plataforma no soportada por install.sh: {platform.system()}/{platform.machine()}")
    return os_name, arch, f"mem_{os_name}_{arch}.tar.gz"


def make_tgz(members: list[tuple[str, bytes | None, str | None]]) -> bytes:
    """members: (nombre, contenido, destino_de_enlace)."""
    import io
    import tarfile
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tar:
        for name, data, link in members:
            info = tarfile.TarInfo(name)
            info.mode = 0o755
            if link is not None:
                info.type = tarfile.SYMTYPE
                info.linkname = link
                tar.addfile(info)
            else:
                info.size = len(data or b"")
                tar.addfile(info, io.BytesIO(data or b""))
    return buf.getvalue()


def sha256_hex(data: bytes) -> str:
    import hashlib
    return hashlib.sha256(data).hexdigest()


def case_installation(bench: Bench) -> list[CaseResult]:
    """US4 / H-04 y H-08 en Unix: matriz de install.sh y reemplazo de update."""
    r: list[CaseResult] = []
    _, _, asset = platform_asset()
    real = bench.mem_bin.read_bytes()
    good = make_tgz([("mem", real, None)])
    previous = b"#!/bin/sh\necho previous\n"
    script = REPO_ROOT / "scripts" / "install.sh"
    system_path = SAFE_SYSTEM_PATH

    restricted = bench.work_dir / "path-sin-sha"
    restricted.mkdir(parents=True, exist_ok=True)
    for tool in INSTALL_TOOLS:
        found = shutil.which(tool, path=system_path)
        link = restricted / tool
        if found and not link.exists():
            link.symlink_to(found)

    def scenario(name: str, *, archive: bytes | None = good, checksums: str | None = None, tag: str = "v9.9.9",
                 version: str | None = None, latest: str | None = None, fail_url: str = "", no_sha: bool = False,
                 readonly_dir: bool = False, omit_checksums: bool = False, expect_ok: bool = False) -> None:
        root = bench.work_dir / "install" / name
        if root.exists():
            shutil.rmtree(root, ignore_errors=True)
        fixture = root / "release"
        (fixture / "download" / tag).mkdir(parents=True)
        if archive is not None:
            (fixture / "download" / tag / asset).write_bytes(archive)
        if checksums is None and archive is not None:
            checksums = f"{sha256_hex(archive)}  {asset}\n"
        if checksums is not None and not omit_checksums:
            (fixture / "download" / tag / "checksums.txt").write_text(checksums)
        if latest is not None:
            (fixture / "api").mkdir()
            (fixture / "api" / "latest.json").write_text(json.dumps({"tag_name": latest}))
        bin_dir = root / "bin"
        bin_dir.mkdir()
        target = bin_dir / "mem"
        target.write_bytes(previous)
        target.chmod(0o755)
        if readonly_dir:
            bin_dir.chmod(0o555)
        path = f"{FAKE_CURL_DIR}:{restricted if no_sha else system_path}"
        env = {**bench.env, "PATH": path, "INSTALL037_FIXTURE": str(fixture), "INSTALL037_FAIL_URL": fail_url,
               "GOMEMORY_BIN_DIR": str(bin_dir), "GOMEMORY_VERSION": version or "latest", "NO_COLOR": "1"}
        proc = subprocess.run(["bash", str(script)], env=env, input="", capture_output=True, text=True, timeout=120,
                              cwd=root)
        if readonly_dir:
            bin_dir.chmod(0o755)
        installed = target.read_bytes()
        leftovers = sorted(p.name for p in bin_dir.iterdir() if p.name != "mem")
        if expect_ok:
            check(r, f"install.{name}", proc.returncode == 0 and installed == real and not leftovers,
                  f"salida {proc.returncode}, instalado={'nuevo' if installed == real else 'otro'}, residuos={leftovers}: {proc.stderr.strip()[-160:]}")
        else:
            check(r, f"install.{name}", proc.returncode != 0 and installed == previous and not leftovers,
                  f"salida {proc.returncode}, previo {'intacto' if installed == previous else 'MODIFICADO'}, residuos={leftovers}")

    digest = sha256_hex(good)
    scenario("explicit_ok", version="v9.9.9", expect_ok=True)
    scenario("latest_resolved_once", latest="v9.9.9", expect_ok=True)
    scenario("hash_mismatch", version="v9.9.9", checksums=f"{'0' * 64}  {asset}\n")
    scenario("checksums_absent", version="v9.9.9", omit_checksums=True)
    scenario("checksums_404", version="v9.9.9", fail_url="checksums.txt")
    scenario("checksums_invalid", version="v9.9.9", checksums=f"zz{digest[2:]}  {asset}\n")
    scenario("entry_missing", version="v9.9.9", checksums=f"{digest}  otro_asset.tar.gz\n")
    scenario("entry_substring_only", version="v9.9.9", checksums=f"{digest}  {asset}.sig\n")
    scenario("entry_duplicated", version="v9.9.9", checksums=f"{digest}  {asset}\n{digest} *{asset}\n")
    scenario("network_failure", version="v9.9.9", fail_url=asset)
    scenario("no_sha_tool", version="v9.9.9", no_sha=True)
    empty = make_tgz([("mem", b"", None)])
    scenario("empty_binary", version="v9.9.9", archive=empty)
    scenario("symlink_member", version="v9.9.9", archive=make_tgz([("mem", None, "/bin/sh")]))
    scenario("traversal_member", version="v9.9.9", archive=make_tgz([("../mem", real, None)]))
    scenario("not_platform_binary", version="v9.9.9", archive=make_tgz([("mem", b"#!/bin/sh\necho x\n", None)]))
    scenario("prerelease_tag", version="v9.9.9-rc1", tag="v9.9.9-rc1")
    if os.geteuid() != 0:
        scenario("readonly_destination", version="v9.9.9", readonly_dir=True)

    r.extend(update_concurrency_probe(bench, asset, good))
    return r


def update_concurrency_probe(bench: Bench, asset: str, archive: bytes) -> list[CaseResult]:
    """mem update contra un release local mientras otras invocaciones usan el
    binario: ninguna debe encontrarlo ausente o incompleto (H-04)."""
    import http.server
    import tempfile
    import threading

    r: list[CaseResult] = []
    files = {
        f"/download/v99.0.0/{asset}": archive,
        "/download/v99.0.0/checksums.txt": f"{sha256_hex(archive)}  {asset}\n".encode(),
        "/repos/Sayoner-000/gomemory/releases/latest": json.dumps({"tag_name": "v99.0.0"}).encode(),
    }

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self) -> None:  # noqa: N802
            body = files.get(self.path)
            self.send_response(200 if body is not None else 404)
            self.end_headers()
            if body is not None:
                self.wfile.write(body)

        def log_message(self, *args) -> None:
            pass

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f"http://127.0.0.1:{server.server_address[1]}"
    upd_dir = bench.work_dir / "update"
    upd_dir.mkdir(parents=True, exist_ok=True)
    target = upd_dir / "mem"
    shutil.copy2(bench.mem_bin, target)
    target.chmod(0o755)
    # El directorio del binario de prueba va primero en PATH: así el binario
    # "global" que resuelve mem update es el mismo archivo del banco y nunca
    # uno de la máquina del operador.
    env = {**bench.env, "PATH": f"{upd_dir}:{SAFE_SYSTEM_PATH}",
           "GOMEMORY_RELEASE_API_BASE": base, "GOMEMORY_RELEASE_DOWNLOAD_BASE": base}

    resolved = shutil.which("mem", path=env["PATH"])
    if resolved is None or not os.path.samefile(resolved, target):
        raise PreparationError(f"mem en el PATH del banco no es el binario de prueba: {resolved}")

    failures: list[str] = []
    probes = 0
    stop = threading.Event()

    def probe() -> None:
        nonlocal probes
        while not stop.is_set():
            try:
                res = subprocess.run([str(target), "version"], env=env, capture_output=True, timeout=10)
                probes += 1
                if res.returncode != 0:
                    failures.append(f"salida {res.returncode}")
            except OSError as exc:
                probes += 1
                failures.append(type(exc).__name__)

    full_size = target.stat().st_size

    def watch() -> None:
        # Observación directa y barata: la ventana del reemplazo antiguo es
        # más corta que el arranque de un proceso, pero no que un stat.
        nonlocal probes
        while not stop.is_set():
            try:
                size = target.stat().st_size
                probes += 1
                if size != full_size:
                    failures.append(f"tamaño parcial {size}")
            except FileNotFoundError:
                probes += 1
                failures.append("ausente")

    workers = [threading.Thread(target=probe) for _ in range(2)] + [threading.Thread(target=watch)]
    for w in workers:
        w.start()
    with tempfile.TemporaryDirectory() as cwd:
        res = subprocess.run([str(target), "update", "--version", "v99.0.0", "--yes"], env=env, cwd=cwd,
                             capture_output=True, text=True, timeout=120)
    stop.set()
    for w in workers:
        w.join()
    server.shutdown()
    leftovers = sorted(p.name for p in upd_dir.iterdir() if p.name != "mem")
    dest_lines = [line for line in res.stdout.splitlines() if "Binario:" in line]
    if dest_lines and not all(str(target) in line for line in dest_lines):
        raise PreparationError(f"mem update apuntó fuera del banco: {dest_lines}")
    check(r, "update.completes", res.returncode == 0 and target.read_bytes() == bench.mem_bin.read_bytes(),
          f"salida {res.returncode}: {res.stderr.strip()[-160:]}")
    check(r, "update.never_absent", probes > 0 and not failures, f"{len(failures)}/{probes} invocaciones fallaron: {failures[:3]}")
    check(r, "update.no_leftovers", not leftovers, f"residuos: {leftovers}")
    return r


def case_wrap(bench: Bench) -> list[CaseResult]:
    """US6 / H-07: arranque fallido frente a hijo iniciado, con stdin cerrado."""
    r: list[CaseResult] = []
    dialog = "¿Guardar?"
    no_exec = bench.work_dir / "sin-permiso.sh"
    no_exec.write_text("#!/bin/sh\necho no\n")
    no_exec.chmod(0o644)
    for name, args in (("missing", ["comando-inexistente-037", f"--token={PRIV}"]), ("no_permission", [str(no_exec)])):
        run = mem(bench, "wrap", "-s=false", *args)
        check(r, f"wrap.{name}", run.code == 1 and "WRAP_START_FAILED" in run.err and dialog not in run.err
              and not leaks(run.out + run.err), f"salida {run.code}: {run.err.strip()[:120]}")
    for name, script, code, out in (("child_0", "echo hola", 0, "hola"), ("child_7", "exit 7", 7, ""), ("child_1", "exit 1", 1, "")):
        run = mem(bench, "wrap", "-s=false", "sh", "-c", script)
        check(r, f"wrap.{name}", run.code == code and out in run.out and dialog in run.err
              and "WRAP_START_FAILED" not in run.err, f"salida {run.code}")
    run = mem(bench, "wrap", "-s=false", "sh", "-c", "kill -TERM $$")
    check(r, "wrap.child_signal", run.code != 0, f"salida {run.code}")
    return r


def not_implemented(name: str) -> Callable[[Bench], list[CaseResult]]:
    def run(_: Bench) -> list[CaseResult]:
        raise PreparationError(f"caso {name} no implementado")

    return run


CASES: dict[str, Callable[[Bench], list[CaseResult]]] = {
    "privacy": case_privacy,
    "history": case_history,
    "context-wrap": lambda bench: case_context(bench) + case_wrap(bench),
    "installation": case_installation,
}


def parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--mem-bin", required=True, type=Path)
    p.add_argument("--work-dir", required=True, type=Path)
    p.add_argument("--data-home", required=True, type=Path)
    p.add_argument("--out-dir", required=True, type=Path)
    p.add_argument("--case", required=True, choices=[*CASES, "all"])
    return p.parse_args(argv)


def write_evidence(out_dir: Path, selected: list[str], results: list[CaseResult], status: str) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    payload = {
        "cases_selected": selected,
        "cases_run": len(results),
        "status": status,
        "results": [r.__dict__ for r in results],
    }
    (out_dir / "result.json").write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n")


def main(argv: list[str]) -> int:
    try:
        preflight()
    except PreparationError as exc:
        print(exc, file=sys.stderr)
        return EXIT_PREPARATION

    args = parse_args(argv)
    selected = list(CASES) if args.case == "all" else [args.case]
    results: list[CaseResult] = []
    try:
        if not args.mem_bin.is_file() or not os.access(args.mem_bin, os.X_OK):
            raise PreparationError(f"--mem-bin no es un ejecutable: {args.mem_bin}")
        args.work_dir.mkdir(parents=True, exist_ok=True)
        base = args.work_dir.resolve()
        env = isolated_env(base / ".bench", args.data_home.resolve())
        for name in selected:
            # Cada caso trabaja en su propio proyecto: comparten almacén
            # aislado, pero no filas ni contexto generado.
            bench = Bench(
                mem_bin=args.mem_bin.resolve(),
                work_dir=base / name,
                data_home=args.data_home.resolve(),
                out_dir=args.out_dir.resolve() / name,
                env=env,
            )
            prepare_project(bench.work_dir)
            results.extend(CASES[name](bench))
    except PreparationError as exc:
        print(f"preparación inválida: {exc}", file=sys.stderr)
        write_evidence(args.out_dir, selected, results, "preparation_failed")
        return EXIT_PREPARATION

    if not results:
        print("cero casos ejecutados", file=sys.stderr)
        write_evidence(args.out_dir, selected, results, "preparation_failed")
        return EXIT_PREPARATION

    failed = [r for r in results if not r.ok]
    for r in results:
        print(f"{'ok  ' if r.ok else 'FAIL'} {r.case_id} {r.detail}".rstrip())
    print(f"{len(results) - len(failed)}/{len(results)} casos aprobados")
    write_evidence(args.out_dir, selected, results, "failed" if failed else "passed")
    return EXIT_DEFECT if failed else EXIT_OK


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
