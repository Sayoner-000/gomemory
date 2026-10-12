"""T019 (feature 038, FR-008–011): pruebas rojas del evaluador offline.

Cada caso usa datos sintéticos o públicos; ninguna traza sale de tmp_path.
El evaluador vive en scripts/validate_038_evidence.py (T020–T022) y solo usa
la biblioteca estándar. Antes de T020, la importación falla: rojo pertinente.
"""
import hashlib
import importlib.util
import json
import sys
from pathlib import Path

import pytest

TEST_FILE = Path(__file__).resolve()
ROOT_HINT = TEST_FILE.parent
EVALUATOR = Path("scripts/validate_038_evidence.py")


def load_evaluator():
    path = ROOT_HINT / EVALUATOR if (ROOT_HINT / EVALUATOR).exists() else Path(EVALUATOR)
    spec = importlib.util.spec_from_file_location("validate_038_evidence", path)
    module = importlib.util.module_from_spec(spec)
    sys.modules["validate_038_evidence"] = module
    spec.loader.exec_module(module)
    return module


def sha(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()


ZERO = "0" * 64


def key(**over):
    base = {
        "tool_family": "search_code",
        "source_scope": "demo-go",
        "source_revision": "r1",
        "query": "funciones de hooks",
        "filters": ["language:go"],
        "limit": 10,
    }
    base.update(over)
    return base


def query(qid="q1", coaching=False, eligible="eligible", **over):
    base = {
        "id": qid,
        "prompt": "Enumera las funciones de hooks y sus ubicaciones",
        "eligibility": eligible,
        "eligibility_reason": "estructural Go con posible reducción",
        "comparison_key": key(),
        "required_facts": [{"id": "f1", "text": "Existe HookA"}],
        "required_references": [{"id": "r1", "text": "hooks.go:10"}],
        "manual_tool_coaching": coaching,
    }
    base.update(over)
    return base


def write_trace(tmp: Path, name: str, text: str) -> tuple[str, str]:
    d = tmp / ".memory" / "validation" / "038"
    d.mkdir(parents=True, exist_ok=True)
    p = d / name
    raw = text.encode("utf-8")
    p.write_bytes(raw)
    return name, sha(raw)


def call(cid, qid, side, text, tmp, trace_name, compression=None, route=None, point="codex_tui_model_input"):
    rel, digest = write_trace(tmp, trace_name, text)
    return {
        "id": cid,
        "query_id": qid,
        "side": side,
        "tool": "search_code",
        "route": route or ("compressed_origin" if side == "observed" else "external"),
        "compression": compression or ("max" if side == "observed" else "none"),
        "arguments": {"query": "funciones de hooks", "limit": 10},
        "comparison_key": key(),
        "visible_text": text,
        "capture": {
            "point": point,
            "session_id": "s1",
            "trace_path": rel,
            "trace_sha256": digest,
            "agent_version": "codex-cli 0.162.1",
            "mcp_sha256": "a" * 64,
            "codec_id": "codex-tui-visible-text-utf8-v1",
        },
    }


def comparison(qid="q1", obs="o1", ref="r1", extra=(), preserved=True):
    return {
        "query_id": qid,
        "paired_calls": [{"observed_call_id": obs, "reference_call_id": ref}],
        "additional_observed_call_ids": list(extra),
        "review": {
            "reviewer": "qa",
            "equivalence_checked": True,
            "required_evidence_preserved": preserved,
            "fact_evidence": [{"item_id": "f1", "observed_call_id": obs}],
            "reference_evidence": [{"item_id": "r1", "observed_call_id": obs}],
        },
    }


def checks(ok=True, cert_rel=".memory/validation/038/capture-certification.json"):
    c = {"passed": ok, "evidence_ref": "t019"}
    out = {
        "policy_consistent": dict(c),
        "migration_preserved": dict(c),
        "compatible_profiles": dict(c),
        "capture_verified": {"passed": ok, "evidence_ref": cert_rel},
    }
    return out


def regressions(ok=True):
    c = {"passed": ok, "evidence_ref": "t019"}
    return {"go_tests": dict(c), "pytest_local": dict(c), "pytest_ci": dict(c), "coverage_80": dict(c)}


def manifest_bytes(*queries):
    return json.dumps({"schema_version": 1, "queries": list(queries)}, ensure_ascii=False).encode("utf-8")


def evidence(tmp: Path, queries, calls, comparisons, session_mode="tui", checks_ok=True, reg_ok=True):
    mb = manifest_bytes(*queries)
    mp = tmp / "manifest.json"
    mp.write_bytes(mb)
    return {
        "schema_version": 1,
        "agent_profile": {
            "agent": "codex",
            "version": "codex-cli 0.162.1",
            "rewrite_status": "unsupported",
            "capability_evidence": "memoria 450",
        },
        "session": {
            "id": "s1",
            "mode": session_mode,
            "repository_revision": "r1",
            "manifest_path": "manifest.json",
            "manifest_sha256": sha(mb),
            "mcp_sha256": "a" * 64,
            "codec_id": "codex-tui-visible-text-utf8-v1",
        },
        "queries": queries,
        "calls": calls,
        "comparisons": comparisons,
        "surface_checks": checks(checks_ok),
        "regression_checks": regressions(reg_ok),
    }


def write_cert(tmp: Path, evidence_mcp="a" * 64, approved=True):
    d = tmp / ".memory" / "validation" / "038"
    d.mkdir(parents=True, exist_ok=True)
    ext = d / "extractor.py"
    ext.write_text("# extractor\n")
    fixture = d / "fixture.txt"
    fixture.write_bytes(" amidst encabezado\nmetadatos\nlínea únicode: ñ\nCRLF\r\naviso\n".encode())
    review_note = d / "review-t024.md"
    review_note.write_text("# Revisión de procedencia T024\nAdaptador y oráculo inspeccionados.\n")
    a_trace = d / "probe-a.trace"
    o_trace = d / "probe-o.trace"
    a_trace.write_bytes(fixture.read_bytes())
    o_trace.write_bytes(fixture.read_bytes())
    cert = {
        "schema_version": 1,
        "agent_version": "codex-cli 0.162.1",
        "mcp_sha256": evidence_mcp,
        "adapter": {
            "interface": "tui-hooks",
            "event": "PostToolUse",
            "field_path": "tool_response.text",
            "implementation_ref": "codex_setup.go",
            "extractor_path": "extractor.py",
            "extractor_sha256": sha(ext.read_bytes()),
        },
        "oracle": {
            "interface": "model-input-log",
            "event": "ModelCall",
            "field_path": "input.items",
            "implementation_ref": "rollout-trace",
            "extractor_path": "extractor.py",
            "extractor_sha256": sha(ext.read_bytes()),
        },
        "probes": [
            {
                "id": "p1", "side": "observed", "session_id": "s1", "call_id": "probe-o",
                "marker": "M-O", "fixture_path": "fixture.txt",
                "fixture_sha256": sha(fixture.read_bytes()),
                "adapter_trace_path": "probe-a.trace",
                "adapter_trace_sha256": sha(a_trace.read_bytes()),
                "oracle_trace_path": "probe-o.trace",
                "oracle_trace_sha256": sha(o_trace.read_bytes()),
            },
            {
                "id": "p2", "side": "reference", "session_id": "s1", "call_id": "probe-r",
                "marker": "M-R", "fixture_path": "fixture.txt",
                "fixture_sha256": sha(fixture.read_bytes()),
                "adapter_trace_path": "probe-a.trace",
                "adapter_trace_sha256": sha(a_trace.read_bytes()),
                "oracle_trace_path": "probe-o.trace",
                "oracle_trace_sha256": sha(o_trace.read_bytes()),
            },
        ],
        "review": {"reviewer": "qa", "approved": approved, "source_review_ref": "review-t024.md"},
    }
    (d / "capture-certification.json").write_text(json.dumps(cert))
    return cert


def run(ev, mod, tmp: Path, monkeypatch):
    monkeypatch.chdir(tmp)
    return mod.evaluate(ev)


@pytest.fixture()
def mod():
    return load_evaluator()


def triplet(tmp: Path, texts):
    """Tres consultas elegibles con pares válidos para aspirar a closed."""
    queries, calls, comps = [], [], []
    for i, (qid, otext, rtext) in enumerate(texts, start=1):
        q = query(qid)
        q["required_facts"] = [{"id": f"f{i}", "text": f"Hecho {qid}"}]
        q["required_references"] = [{"id": f"r{i}", "text": f"ref {qid}"}]
        queries.append(q)
        calls.append(call(f"o{i}", qid, "observed", otext, tmp, f"o{i}.trace"))
        calls.append(call(f"r{i}", qid, "reference", rtext, tmp, f"r{i}.trace"))
        comps.append(
            {
                "query_id": qid,
                "paired_calls": [{"observed_call_id": f"o{i}", "reference_call_id": f"r{i}"}],
                "additional_observed_call_ids": [],
                "review": {
                    "reviewer": "qa",
                    "equivalence_checked": True,
                    "required_evidence_preserved": True,
                    "fact_evidence": [{"item_id": f"f{i}", "observed_call_id": f"o{i}"}],
                    "reference_evidence": [{"item_id": f"r{i}", "observed_call_id": f"o{i}"}],
                },
            }
        )
    return queries, calls, comps


def valid_triplet(tmp, monkeypatch, mod):
    write_cert(tmp)
    texts = [
        ("q1", "[1] function HookA (pkg) — hooks.go:10\n", "[1] function HookA (pkg) — hooks.go:10\nfunc HookA() {}\n"),
        ("q2", "[2] function HookB (pkg) — ctx.go:4\n", "[2] function HookB (pkg) — ctx.go:4\nfunc HookB() {}\n"),
        ("q3", "[3] function HookC (pkg) — out.go:7\n", "[3] function HookC (pkg) — out.go:7\nfunc HookC() {}\n"),
    ]
    queries, calls, comps = triplet(tmp, texts)
    ev = evidence(tmp, queries, calls, comps)
    return ev, mod


def test_cierre_con_ahorro_valido(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "closed"
    assert rep["aggregate_reference_bytes"] > rep["aggregate_observed_bytes"]
    assert rep["delta_bytes"] > 0


def test_overhead_anula_ahorro(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    big = "[1] function HookA (pkg) — hooks.go:10\n" + "x" * 500 + "\n"
    small = "[1] function HookA (pkg) — hooks.go:10\n"
    calls = [
        call("o1", "q1", "observed", big, tmp_path, "o1.trace"),
        call("r1", "q1", "reference", small, tmp_path, "r1.trace"),
    ]
    ev = evidence(tmp_path, [query()], calls, [comparison()])
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "pending"


def test_recuperaciones_adicionales_anulan(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    calls = [
        call("o1", "q1", "observed", "A hooks.go:10\n", tmp_path, "o1.trace"),
        call("o2", "q1", "observed", "B" * 500 + "\n", tmp_path, "o2.trace"),
        call("r1", "q1", "reference", "A hooks.go:10\n", tmp_path, "r1.trace"),
    ]
    comp = comparison(extra=["o2"])
    ev = evidence(tmp_path, [query()], calls, [comp])
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "pending"


def test_unicode_crlf_sin_normalizar(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    texts = [
        ("q1", "Encabezado\r\nFunción Héllo — ubicación\r\naviso\r\n", "Encabezado\r\nFunción Héllo — ubicación\r\naviso\r\ncuerpo extra\n"),
        ("q2", "Encabezado\r\nFunción Wörld — ubicación\r\naviso\r\n", "Encabezado\r\nFunción Wörld — ubicación\r\naviso\r\ncuerpo extra\n"),
        ("q3", "Encabezado\r\nFunción Ñandú — ubicación\r\naviso\r\n", "Encabezado\r\nFunción Ñandú — ubicación\r\naviso\r\ncuerpo extra\n"),
    ]
    queries, calls, comps = triplet(tmp_path, texts)
    ev = evidence(tmp_path, queries, calls, comps)
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "closed"
    first = texts[0][1].encode("utf-8")
    assert rep["comparisons"][0]["observed_complete_bytes"] == len(first)


def test_delta_negativo_y_nulo(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    calls = [
        call("o1", "q1", "observed", "AA\n", tmp_path, "o1.trace"),
        call("r1", "q1", "reference", "A\n", tmp_path, "r1.trace"),
    ]
    ev = evidence(tmp_path, [query()], calls, [comparison()])
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "pending"
    assert rep["delta_bytes"] < 0


def test_firma_no_equivale_a_cuerpo(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    calls = [
        call("o1", "q1", "observed", "HookA hooks.go:10\n", tmp_path, "o1.trace"),
        call("r1", "q1", "reference", "HookA hooks.go:10\nfunc HookA() {}\n", tmp_path, "r1.trace"),
    ]
    q = query()
    q["required_facts"] = [{"id": "f1", "text": "cuerpo literal de HookA"}]
    ev = evidence(tmp_path, [q], calls, [comparison()])
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] in ("pending", "blocked")


def test_perdida_de_hecho_obligatorio(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["comparisons"][0]["review"]["fact_evidence"] = []
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "pending"


def test_claves_de_comparacion_distintas(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["calls"][0]["comparison_key"]["limit"] = 5
    assert run(ev, mod, tmp_path, monkeypatch)["status"] in ("pending", "blocked")


def resync_manifest(tmp: Path, ev):
    raw = manifest_bytes(*ev["queries"])
    (tmp / "manifest.json").write_bytes(raw)
    ev["session"]["manifest_sha256"] = sha(raw)


def test_coaching_invalida_cierre(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["queries"][0]["manual_tool_coaching"] = True
    resync_manifest(tmp_path, ev)
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "pending"


def test_sesion_exec_no_cierra(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    calls = [
        call("o1", "q1", "observed", "A\n", tmp_path, "o1.trace"),
        call("r1", "q1", "reference", "AA\n", tmp_path, "r1.trace"),
    ]
    ev = evidence(tmp_path, [query()], calls, [comparison()], session_mode="exec")
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_metricas_historicas_no_sustituyen(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["session"]["mode"] = "synthetic"
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_captura_incompleta_bloquea(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["calls"][0]["capture"]["point"] = "unsupported"
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_captura_no_textual_bloquea(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    d = tmp_path / ".memory" / "validation" / "038"
    raw = b"\xff\xfe binario \x00"
    (d / "o1.trace").write_bytes(raw)
    calls = [
        dict(call("o1", "q1", "observed", "A\n", tmp_path, "o1b.trace")),
        call("r1", "q1", "reference", "AA\n", tmp_path, "r1.trace"),
    ]
    calls[0]["capture"]["trace_path"] = "o1.trace"
    calls[0]["capture"]["trace_sha256"] = sha(raw)
    ev = evidence(tmp_path, [query()], calls, [comparison()])
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_json_con_clave_duplicada(mod):
    dup = '{"schema_version": 1, "schema_version": 2}'
    with pytest.raises(ValueError):
        mod.parse_strict(dup)


def test_capture_verified_con_ruta_distinta(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["surface_checks"]["capture_verified"] = {"passed": True, "evidence_ref": "otra/ruta.json"}
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_certificado_ausente_bloquea(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    (tmp_path / ".memory" / "validation" / "038" / "capture-certification.json").unlink()
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_certificado_corrupto_bloquea(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    (tmp_path / ".memory" / "validation" / "038" / "capture-certification.json").write_text("{no json")
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_certificado_sin_aprobar_bloquea_cierre(tmp_path, monkeypatch, mod):
    write_cert(tmp_path, approved=False)
    calls = [
        call("o1", "q1", "observed", "A\n", tmp_path, "o1.trace"),
        call("r1", "q1", "reference", "AA\n", tmp_path, "r1.trace"),
    ]
    ev = evidence(tmp_path, [query()], calls, [comparison()])
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_lados_incompletos_bloquean(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["comparisons"][0]["paired_calls"] = []
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_trazas_divergentes_bloquean(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    d = tmp_path / ".memory" / "validation" / "038"
    (d / "probe-o.trace").write_bytes(b"oraculo distinto")
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_tres_consultas_minimas(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    calls = [
        call("o1", "q1", "observed", "A\n", tmp_path, "o1.trace"),
        call("r1", "q1", "reference", "AA\n", tmp_path, "r1.trace"),
    ]
    ev = evidence(tmp_path, [query()], calls, [comparison()])
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "pending"


def test_ruta_con_escape_bloquea(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    (tmp_path / "fuera.trace").write_bytes(b"fuera del banco\n")
    ev["calls"][0]["capture"]["trace_path"] = "../fuera.trace"
    ev["calls"][0]["capture"]["trace_sha256"] = sha(b"fuera del banco\n")
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_llamada_sin_comparar_bloquea(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    extra = call("ox", "q1", "observed", "B" * 500 + "\n", tmp_path, "ox.trace")
    ev["calls"].append(extra)
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_hechos_duplicados_bloquean(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["queries"][0]["required_facts"] = [
        {"id": "f1", "text": "Existe HookA"},
        {"id": "f1", "text": "Existe HookA otra vez"},
    ]
    resync_manifest(tmp_path, ev)
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


def test_probes_con_marcador_repetido_bloquean(tmp_path, monkeypatch, mod):
    write_cert(tmp_path)
    d = tmp_path / ".memory" / "validation" / "038"
    cert = json.loads((d / "capture-certification.json").read_text())
    cert["probes"][1]["marker"] = cert["probes"][0]["marker"]
    (d / "capture-certification.json").write_text(json.dumps(cert))
    calls = [
        call("o1", "q1", "observed", "A\n", tmp_path, "o1.trace"),
        call("r1", "q1", "reference", "AA\n", tmp_path, "r1.trace"),
    ]
    ev = evidence(tmp_path, [query()], calls, [comparison()])
    assert run(ev, mod, tmp_path, monkeypatch)["status"] == "blocked"


# --- Auditoría QA 2026-10-12: causa por motivo, enlaces y E/S (FR-010) ---


def test_coaching_se_atribuye_a_ausencia_de_uso(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["queries"][0]["manual_tool_coaching"] = True
    resync_manifest(tmp_path, ev)
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["cause"] == [mod.CAUSE_USO]


def test_reescritura_no_admitida_es_incompatibilidad(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["calls"][0]["route"] = "automatic_rewrite"
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "pending"
    assert rep["cause"] == [mod.CAUSE_INCOMPATIBILIDAD]


def test_capacidad_desconocida_es_medicion(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["agent_profile"]["rewrite_status"] = "unknown"
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["cause"] == [mod.CAUSE_MEDICION]


def test_id_de_consulta_no_contamina_la_causa(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["comparisons"][0]["review"]["equivalence_checked"] = False
    for q in ev["queries"]:
        if q["id"] == "q1":
            q["id"] = "ruta-coaching-surface"
    for c in ev["calls"]:
        if c["query_id"] == "q1":
            c["query_id"] = "ruta-coaching-surface"
    ev["comparisons"][0]["query_id"] = "ruta-coaching-surface"
    resync_manifest(tmp_path, ev)
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["cause"] == [mod.CAUSE_RECUPERACION]


def test_check_fallido_es_instrucciones(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["regression_checks"]["go_tests"]["passed"] = False
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["cause"] == [mod.CAUSE_INSTRUCCIONES]


def test_traza_enlazada_simbolicamente_bloquea(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    bank = tmp_path / ".memory" / "validation" / "038"
    (bank / "enlace.trace").symlink_to(bank / "o1.trace")
    ev["calls"][0]["capture"]["trace_path"] = "enlace.trace"
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "blocked"
    assert "fuera del banco" in rep["failure_reasons"][0]


def test_error_de_lectura_bloquea_con_informe(tmp_path, monkeypatch, mod):
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    real = Path.read_bytes

    def falla(self):
        if self.name == "o1.trace":
            raise PermissionError("sin permiso")
        return real(self)

    monkeypatch.setattr(Path, "read_bytes", falla)
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "blocked"
    assert "ilegible (PermissionError)" in rep["failure_reasons"][0]


def test_elegible_sin_comparacion_queda_pendiente(tmp_path, monkeypatch, mod):
    # ACR ronda 2 C-006: tres pares válidos no bastan si se omite una elegible.
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    ev["queries"].append(query("q4"))
    resync_manifest(tmp_path, ev)
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "pending"
    assert "consultas elegibles sin comparación: ['q4']" in rep["failure_reasons"]
    assert rep["cause"] == [mod.CAUSE_MEDICION]


def test_filters_en_otro_orden_no_son_equivalentes(tmp_path, monkeypatch, mod):
    # ACR ronda 2 C-005: filters es lista ordenada; un par desordenado bloquea.
    ev, mod = valid_triplet(tmp_path, monkeypatch, mod)
    for c in ev["calls"][:2]:
        c["comparison_key"]["filters"] = ["language:go", "kind:function"]
    ev["calls"][1]["comparison_key"]["filters"].reverse()
    rep = run(ev, mod, tmp_path, monkeypatch)
    assert rep["status"] == "blocked"
    assert "claves de comparación distintas" in rep["failure_reasons"][0]
