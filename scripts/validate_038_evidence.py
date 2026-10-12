"""Validador offline de evidencia 038 (solo biblioteca estándar).

Uso:
    python3 scripts/validate_038_evidence.py --input evidence.json --output report.json

Estados: closed (exit 0), pending (exit 1), blocked (exit 2).
No ejecuta texto capturado, no accede a la red, no importa nada externo.
"""
import argparse
import hashlib
import json
import sys
from pathlib import Path

CODEC = "codex-tui-visible-text-utf8-v1"
CERT_REL = Path(".memory/validation/038/capture-certification.json")
TRACE_ROOT = Path(".memory/validation/038")
REPORT_VERSION = 1

EXIT = {"closed": 0, "pending": 1, "blocked": 2}

CAUSE_MEDICION = "medición insuficiente"
CAUSE_USO = "ausencia de uso"
CAUSE_INSTRUCCIONES = "instrucciones contradictorias"
CAUSE_RECUPERACION = "fallo de recuperación"
CAUSE_INCOMPATIBILIDAD = "incompatibilidad del agente"


class Blocked(Exception):
    def __init__(self, reason, cause=CAUSE_MEDICION):
        super().__init__(reason)
        self.reason = reason
        self.cause = cause


def _dup_guard(pairs):
    obj = {}
    for k, v in pairs:
        if k in obj:
            raise ValueError(f"clave JSON duplicada: {k!r}")
        obj[k] = v
    return obj


def _no_const(value):
    raise ValueError("literal no permitido en JSON estricto")


def parse_strict(text):
    """Analiza JSON UTF-8 estricto: rechaza claves duplicadas y NaN/Infinity."""
    try:
        return json.loads(
            text,
            object_pairs_hook=_dup_guard,
            parse_constant=_no_const,
        )
    except ValueError:
        raise
    except Exception as exc:  # pragma: no cover - defensivo
        raise ValueError(f"JSON inválido: {exc}")


def parse_bytes(raw):
    try:
        return parse_strict(raw.decode("utf-8"))
    except UnicodeDecodeError as exc:
        raise ValueError(f"UTF-8 inválido: {exc}")


def _is_sha(v):
    return isinstance(v, str) and len(v) == 64 and all(c in "0123456789abcdef" for c in v)


def _is_id(v):
    return isinstance(v, str) and len(v) > 0


def _check_key(k, path, errors):
    if not isinstance(k, dict):
        errors.append(f"{path}: objeto esperado")
        return
    allowed = {"tool_family", "source_scope", "source_revision", "query", "filters", "limit"}
    if set(k) != allowed:
        errors.append(f"{path}: claves {sorted(k)} != {sorted(allowed)}")
        return
    for f in ("tool_family", "source_scope", "source_revision", "query"):
        if not isinstance(k[f], str) or not k[f]:
            errors.append(f"{path}.{f}: string no vacío esperado")
    if not isinstance(k["filters"], list) or any(not isinstance(x, str) for x in k["filters"]):
        errors.append(f"{path}.filters: lista ordenada de strings esperada")
    if not isinstance(k["limit"], int) or isinstance(k["limit"], bool) or k["limit"] < 1:
        errors.append(f"{path}.limit: entero positivo esperado")


def check_schema(ev):
    """Valida la forma v1. Devuelve lista de errores (vacía = válida)."""
    errors = []
    if not isinstance(ev, dict):
        return ["raíz: objeto esperado"]
    want_root = {"schema_version", "agent_profile", "session", "queries", "calls",
                 "comparisons", "surface_checks", "regression_checks"}
    if set(ev) != want_root:
        return [f"raíz: claves {sorted(ev)} != {sorted(want_root)}"]
    if ev["schema_version"] != 1:
        errors.append("schema_version: debe ser 1")

    ap = ev["agent_profile"]
    if not isinstance(ap, dict) or set(ap) != {"agent", "version", "rewrite_status", "capability_evidence"}:
        errors.append("agent_profile: forma inválida")
    else:
        if not _is_id(ap.get("agent")) or not _is_id(ap.get("version")):
            errors.append("agent_profile: agent/version no vacíos")
        if ap.get("rewrite_status") not in ("supported", "unsupported", "unknown"):
            errors.append("agent_profile.rewrite_status: valor inválido")
        if not isinstance(ap.get("capability_evidence"), str) or not ap["capability_evidence"]:
            errors.append("agent_profile.capability_evidence: string no vacío")

    se = ev["session"]
    if not isinstance(se, dict) or set(se) != {"id", "mode", "repository_revision", "manifest_path",
                                               "manifest_sha256", "mcp_sha256", "codec_id"}:
        errors.append("session: forma inválida")
    else:
        if not _is_id(se.get("id")):
            errors.append("session.id: identificador no vacío")
        if se.get("mode") not in ("tui", "exec", "synthetic"):
            errors.append("session.mode: valor inválido")
        if not isinstance(se.get("repository_revision"), str) or not se["repository_revision"]:
            errors.append("session.repository_revision: string no vacío")
        if not isinstance(se.get("manifest_path"), str) or not se["manifest_path"]:
            errors.append("session.manifest_path: string no vacío")
        if not _is_sha(se.get("manifest_sha256")) or not _is_sha(se.get("mcp_sha256")):
            errors.append("session: digests SHA-256 hexadecimales esperados")
        if se.get("codec_id") != CODEC:
            errors.append(f"session.codec_id: debe ser {CODEC}")

    if not isinstance(ev["queries"], list) or not ev["queries"]:
        errors.append("queries: lista no vacía esperada")
    else:
        for i, q in enumerate(ev["queries"]):
            p = f"queries[{i}]"
            if not isinstance(q, dict) or set(q) != {"id", "prompt", "eligibility", "eligibility_reason",
                                                     "comparison_key", "required_facts", "required_references",
                                                     "manual_tool_coaching"}:
                errors.append(f"{p}: forma inválida")
                continue
            if not _is_id(q["id"]) or not isinstance(q["prompt"], str) or not q["prompt"]:
                errors.append(f"{p}: id/prompt no vacíos")
            if q["eligibility"] not in ("eligible", "ineligible"):
                errors.append(f"{p}.eligibility: valor inválido")
            if not isinstance(q["eligibility_reason"], str):
                errors.append(f"{p}.eligibility_reason: string esperado")
            _check_key(q["comparison_key"], f"{p}.comparison_key", errors)
            for campo in ("required_facts", "required_references"):
                items = q[campo]
                if not isinstance(items, list) or not items:
                    errors.append(f"{p}.{campo}: lista no vacía esperada")
                elif any(not isinstance(x, dict) or set(x) != {"id", "text"}
                         or not _is_id(x["id"]) or not isinstance(x["text"], str) or not x["text"]
                         for x in items):
                    errors.append(f"{p}.{campo}: objetos {{id, text}} no vacíos")
                elif len({x["id"] for x in items}) != len(items):
                    errors.append(f"{p}.{campo}: IDs duplicados")
            if not isinstance(q["manual_tool_coaching"], bool):
                errors.append(f"{p}.manual_tool_coaching: booleano esperado")

    if not isinstance(ev["calls"], list):
        errors.append("calls: lista esperada")
    else:
        for i, c in enumerate(ev["calls"]):
            p = f"calls[{i}]"
            if not isinstance(c, dict) or set(c) != {"id", "query_id", "side", "tool", "route",
                                                     "compression", "arguments", "comparison_key",
                                                     "visible_text", "capture"}:
                errors.append(f"{p}: forma inválida")
                continue
            if not _is_id(c["id"]) or not _is_id(c["query_id"]) or not _is_id(c["tool"]):
                errors.append(f"{p}: id/query_id/tool no vacíos")
            if c["side"] not in ("observed", "reference"):
                errors.append(f"{p}.side: valor inválido")
            if c["route"] not in ("compressed_origin", "automatic_rewrite", "external", "other"):
                errors.append(f"{p}.route: valor inválido")
            if c["compression"] not in ("max", "none"):
                errors.append(f"{p}.compression: valor inválido")
            if not isinstance(c["arguments"], dict):
                errors.append(f"{p}.arguments: objeto esperado")
            _check_key(c["comparison_key"], f"{p}.comparison_key", errors)
            if not isinstance(c["visible_text"], str):
                errors.append(f"{p}.visible_text: string esperado")
            cap = c["capture"]
            if not isinstance(cap, dict) or set(cap) != {"point", "session_id", "trace_path",
                                                          "trace_sha256", "agent_version",
                                                          "mcp_sha256", "codec_id"}:
                errors.append(f"{p}.capture: forma inválida")
            else:
                if cap.get("point") not in ("codex_tui_model_input", "unsupported"):
                    errors.append(f"{p}.capture.point: valor inválido")
                if not _is_id(cap.get("session_id")):
                    errors.append(f"{p}.capture.session_id: identificador no vacío")
                if not isinstance(cap.get("trace_path"), str) or not cap["trace_path"]:
                    errors.append(f"{p}.capture.trace_path: string no vacío")
                if not _is_sha(cap.get("trace_sha256")) or not _is_sha(cap.get("mcp_sha256")):
                    errors.append(f"{p}.capture: digests SHA-256 esperados")
                if not _is_id(cap.get("agent_version")):
                    errors.append(f"{p}.capture.agent_version: no vacío")
                if cap.get("codec_id") != CODEC:
                    errors.append(f"{p}.capture.codec_id: debe ser {CODEC}")

    if not isinstance(ev["comparisons"], list):
        errors.append("comparisons: lista esperada")
    else:
        for i, comp in enumerate(ev["comparisons"]):
            p = f"comparisons[{i}]"
            if not isinstance(comp, dict) or set(comp) != {"query_id", "paired_calls",
                                                            "additional_observed_call_ids", "review"}:
                errors.append(f"{p}: forma inválida")
                continue
            if not _is_id(comp["query_id"]):
                errors.append(f"{p}.query_id: identificador no vacío")
            pares = comp["paired_calls"]
            if not isinstance(pares, list) or not pares:
                errors.append(f"{p}.paired_calls: lista no vacía esperada")
            elif any(not isinstance(x, dict) or set(x) != {"observed_call_id", "reference_call_id"}
                     or not _is_id(x["observed_call_id"]) or not _is_id(x["reference_call_id"])
                     for x in pares):
                errors.append(f"{p}.paired_calls: pares de IDs no vacíos")
            if not isinstance(comp["additional_observed_call_ids"], list) or \
                    any(not _is_id(x) for x in comp["additional_observed_call_ids"]):
                errors.append(f"{p}.additional_observed_call_ids: lista de IDs esperada")
            rev = comp["review"]
            if not isinstance(rev, dict) or set(rev) != {"reviewer", "equivalence_checked",
                                                         "required_evidence_preserved",
                                                         "fact_evidence", "reference_evidence"}:
                errors.append(f"{p}.review: forma inválida")
            else:
                if not _is_id(rev["reviewer"]):
                    errors.append(f"{p}.review.reviewer: no vacío")
                if not isinstance(rev["equivalence_checked"], bool) or \
                        not isinstance(rev["required_evidence_preserved"], bool):
                    errors.append(f"{p}.review: booleanos esperados")
                for campo in ("fact_evidence", "reference_evidence"):
                    if not isinstance(rev[campo], list) or any(
                            not isinstance(x, dict) or set(x) != {"item_id", "observed_call_id"}
                            or not _is_id(x["item_id"]) or not _is_id(x["observed_call_id"])
                            for x in rev[campo]):
                        errors.append(f"{p}.review.{campo}: lista de {{item_id, observed_call_id}}")

    for grupo, fijos in (("surface_checks", {"policy_consistent", "migration_preserved",
                                              "compatible_profiles", "capture_verified"}),
                          ("regression_checks", {"go_tests", "pytest_local", "pytest_ci", "coverage_80"})):
        g = ev[grupo]
        if not isinstance(g, dict) or set(g) != fijos:
            errors.append(f"{grupo}: forma inválida")
            continue
        for nombre, chk in g.items():
            if not isinstance(chk, dict) or set(chk) != {"passed", "evidence_ref"}:
                errors.append(f"{grupo}.{nombre}: forma inválida")
            elif not isinstance(chk["passed"], bool) or not isinstance(chk["evidence_ref"], str) \
                    or not chk["evidence_ref"]:
                errors.append(f"{grupo}.{nombre}: {{passed: bool, evidence_ref: no vacío}}")
    return errors


def _has_symlink(root, p):
    """Indica si p o algún directorio suyo bajo root es un enlace simbólico.

    Se comprueba ANTES de resolve(): tras resolver, is_symlink() siempre es
    falso y la comprobación no tendría efecto.
    """
    for part in (p, *p.parents):
        if part == root:
            return False
        if part.is_symlink():
            return True
    return False


def _confine(root, base, rel):
    p = base / rel
    if _has_symlink(root, p):
        return None
    try:
        res = p.resolve()
        res.relative_to(base.resolve())
    except (OSError, ValueError):
        return None
    return res


def _resolve_inside(root, rel):
    """Confina una ruta al repositorio (válido para manifest_path y el certificado)."""
    return _confine(root, root, rel)


def _resolve_in_bank(root, rel):
    """Resuelve una ruta del banco dentro de TRACE_ROOT (C-005).

    A diferencia de _resolve_inside (que solo confina al repositorio, válido
    para manifest_path), aquí un `../` que salga del banco se rechaza aunque
    siga dentro del repo.
    """
    return _confine(root, root / TRACE_ROOT, rel)


def _read(path, what):
    """Lee bytes; un error de E/S bloquea con informe en vez de un traceback."""
    try:
        return path.read_bytes()
    except OSError as exc:
        raise Blocked(f"{what}: ilegible ({exc.__class__.__name__})")


def check_cert(cert, root, ev):
    """Valida el certificado de captura. Devuelve lista de errores."""
    errors = []

    def need(cond, msg):
        if not cond:
            errors.append(msg)

    if not isinstance(cert, dict):
        return ["certificado: objeto esperado"]
    if set(cert) != {"schema_version", "agent_version", "mcp_sha256", "adapter", "oracle", "probes", "review"}:
        return ["certificado: forma inválida"]
    need(cert["schema_version"] == 1, "certificado.schema_version: debe ser 1")
    need(_is_id(cert["agent_version"]), "certificado.agent_version: no vacío")
    need(_is_sha(cert["mcp_sha256"]), "certificado.mcp_sha256: SHA-256 esperado")
    need(cert["agent_version"] == ev["agent_profile"]["version"], "certificado: agent_version difiere del perfil")
    need(cert["mcp_sha256"] == ev["session"]["mcp_sha256"], "certificado: mcp_sha256 difiere de la sesión")

    for lado in ("adapter", "oracle"):
        src = cert[lado]
        ok = isinstance(src, dict) and set(src) == {"interface", "event", "field_path",
                                                    "implementation_ref", "extractor_path", "extractor_sha256"}
        need(ok, f"certificado.{lado}: forma inválida")
        if not ok:
            continue
        for campo in ("interface", "event", "field_path", "implementation_ref"):
            need(isinstance(src[campo], str) and src[campo], f"certificado.{lado}.{campo}: no vacío")
        need(_is_sha(src["extractor_sha256"]), f"certificado.{lado}.extractor_sha256: SHA-256")
        ep = _resolve_in_bank(root, src["extractor_path"])
        need(ep is not None and ep.is_file(), f"certificado.{lado}: extractor fuera de {TRACE_ROOT} o ausente")
        if ep is not None and ep.is_file():
            need(hashlib.sha256(_read(ep, f"certificado.{lado}.extractor")).hexdigest() == src["extractor_sha256"],
                 f"certificado.{lado}: hash del extractor no coincide")
    if isinstance(cert.get("adapter"), dict) and isinstance(cert.get("oracle"), dict):
        ta = (cert["adapter"].get("interface"), cert["adapter"].get("event"), cert["adapter"].get("field_path"))
        to = (cert["oracle"].get("interface"), cert["oracle"].get("event"), cert["oracle"].get("field_path"))
        need(ta != to, "certificado: adaptador y oráculo deben ser tuplas (interface,event,field_path) distintas")

    probes = cert.get("probes")
    if not isinstance(probes, list) or len(probes) < 2:
        errors.append("certificado.probes: al menos 2 probes")
        return errors
    sides = set()
    vistos = {"id": set(), "call_id": set(), "marker": set()}
    for i, pr in enumerate(probes):
        p = f"certificado.probes[{i}]"
        want = {"id", "side", "session_id", "call_id", "marker", "fixture_path", "fixture_sha256",
                "adapter_trace_path", "adapter_trace_sha256", "oracle_trace_path", "oracle_trace_sha256"}
        if not isinstance(pr, dict) or set(pr) != want:
            errors.append(f"{p}: forma inválida")
            continue
        if pr["side"] not in ("observed", "reference"):
            errors.append(f"{p}.side: valor inválido")
        sides.add(pr["side"])
        for campo in ("id", "session_id", "call_id", "marker"):
            if not _is_id(pr[campo]):
                errors.append(f"{p}.{campo}: no vacío")
        for campo in ("id", "call_id", "marker"):
            if pr[campo] in vistos[campo]:
                errors.append(f"{p}.{campo}: duplicado entre probes")
            vistos[campo].add(pr[campo])
        for campo, digest in (("fixture_path", "fixture_sha256"),
                              ("adapter_trace_path", "adapter_trace_sha256"),
                              ("oracle_trace_path", "oracle_trace_sha256")):
            need(_is_sha(pr[digest]), f"{p}.{digest}: SHA-256")
            fp = _resolve_in_bank(root, pr[campo])
            need(fp is not None and fp.is_file(), f"{p}.{campo}: fuera de {TRACE_ROOT} o ausente")
            if fp is not None and fp.is_file() and _is_sha(pr[digest]):
                need(hashlib.sha256(_read(fp, f"{p}.{campo}")).hexdigest() == pr[digest],
                     f"{p}.{campo}: hash no coincide")
        aa = _resolve_in_bank(root, pr["adapter_trace_path"])
        oa = _resolve_in_bank(root, pr["oracle_trace_path"])
        if all(x is not None and x.is_file() for x in (aa, oa)):
            need(_read(aa, f"{p}.adapter_trace_path") == _read(oa, f"{p}.oracle_trace_path"), f"{p}: adaptador y oráculo divergen byte a byte")
    need(sides >= {"observed", "reference"}, "certificado.probes: deben cubrir ambos lados")

    rev = cert.get("review")
    ok = isinstance(rev, dict) and set(rev) == {"reviewer", "approved", "source_review_ref"}
    need(ok, "certificado.review: forma inválida")
    if ok:
        need(_is_id(rev["reviewer"]), "certificado.review.reviewer: no vacío")
        need(rev["approved"] is True, "certificado.review.approved: debe ser true")
        rp = _resolve_in_bank(root, rev["source_review_ref"])
        need(rp is not None and rp.is_file(), "certificado.review: referencia de revisión ausente o fuera del banco")
    return errors


def evaluate(ev, root=None):
    """Evalúa una entrada ya analizada. Siempre devuelve el informe (dict)."""
    try:
        return _evaluate(ev, root)
    except Blocked as exc:
        return {"report_version": REPORT_VERSION, "status": "blocked",
                "failure_reasons": [exc.reason], "cause": [exc.cause]}


def _evaluate(ev, root=None):
    root = Path(root) if root is not None else Path.cwd()
    # Cada motivo lleva su causa desde el origen (FR-010): deducirla después
    # buscando subcadenas en el texto clasificaba mal motivos e IDs arbitrarios.
    pending = []

    def pend(reason, cause):
        pending.append((reason, cause))

    def blocked(reason, cause=CAUSE_MEDICION):
        raise Blocked(reason, cause)

    schema_errors = check_schema(ev)
    if schema_errors:
        blocked(f"esquema: {schema_errors[0]}")

    queries = {q["id"]: q for q in ev["queries"]}
    if len(queries) != len(ev["queries"]):
        blocked("queries: IDs duplicados")
    calls = {c["id"]: c for c in ev["calls"]}
    if len(calls) != len(ev["calls"]):
        blocked("calls: IDs duplicados")
    comps = {}
    for comp in ev["comparisons"]:
        if comp["query_id"] in comps:
            blocked("comparisons: query_id duplicado")
        comps[comp["query_id"]] = comp

    session = ev["session"]
    if session["mode"] != "tui":
        blocked(f"session.mode={session['mode']}: solo tui puede cerrar", CAUSE_MEDICION)

    profile = ev["agent_profile"]
    mp = _resolve_inside(root, session["manifest_path"])
    if mp is None or not mp.is_file():
        blocked("manifiesto: ruta fuera del repositorio o ausente")
    # Una sola lectura: el hash y el análisis deben cubrir los mismos bytes.
    raw_manifest = _read(mp, "manifiesto")
    if hashlib.sha256(raw_manifest).hexdigest() != session["manifest_sha256"]:
        blocked("manifiesto: SHA-256 no coincide")
    try:
        manifest = parse_bytes(raw_manifest)
    except ValueError as exc:
        blocked(f"manifiesto ilegible: {exc}")
    if not isinstance(manifest, dict) or manifest.get("schema_version") != 1 \
            or manifest.get("queries") != ev["queries"]:
        blocked("manifiesto: queries difieren de la entrada")

    for c in ev["calls"]:
        if c["query_id"] not in queries:
            blocked(f"call {c['id']}: query_id huérfano")
        cap = c["capture"]
        if cap["point"] != "codex_tui_model_input":
            blocked(f"call {c['id']}: punto de captura no verificado")
        if cap["codec_id"] != session["codec_id"]:
            blocked(f"call {c['id']}: codec difiere de la sesión")
        if cap["agent_version"] != profile["version"]:
            blocked(f"call {c['id']}: agent_version difiere del perfil")
        if cap["mcp_sha256"] != session["mcp_sha256"]:
            blocked(f"call {c['id']}: mcp_sha256 difiere de la sesión")
        tp = _resolve_in_bank(root, cap["trace_path"])
        if tp is None or not tp.is_file():
            blocked(f"call {c['id']}: traza fuera del banco o ausente")
        raw = _read(tp, f"call {c['id']}: traza")
        if hashlib.sha256(raw).hexdigest() != cap["trace_sha256"]:
            blocked(f"call {c['id']}: SHA-256 de traza no coincide")
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError:
            blocked(f"call {c['id']}: traza no es UTF-8 estricto")
        if text != c["visible_text"]:
            blocked(f"call {c['id']}: visible_text difiere de la traza")
        if len(raw) != len(c["visible_text"].encode("utf-8")):
            blocked(f"call {c['id']}: conteo de bytes incoherente")
        if c["side"] == "reference" and c["compression"] != "none":
            blocked(f"call {c['id']}: la referencia usa compression=none")

    cap_check = ev["surface_checks"]["capture_verified"]
    if cap_check["evidence_ref"] != str(CERT_REL):
        blocked("capture_verified: evidence_ref debe ser la ruta fijada")
    cert_path = _resolve_inside(root, CERT_REL)
    if cert_path is None or not cert_path.is_file():
        blocked("capture_verified: certificado ausente")
    try:
        cert = parse_bytes(_read(cert_path, "capture_verified: certificado"))
    except ValueError:
        blocked("capture_verified: certificado ilegible")
    cert_errors = check_cert(cert, root, ev)
    if cert_errors:
        blocked(f"certificado: {cert_errors[0]}")

    usados = {}
    for comp in ev["comparisons"]:
        qid = comp["query_id"]
        if qid not in queries:
            blocked(f"comparison {qid}: consulta inexistente")
        q = queries[qid]
        for par in comp["paired_calls"]:
            for cid, lado in ((par["observed_call_id"], "observed"), (par["reference_call_id"], "reference")):
                if cid not in calls:
                    blocked(f"comparison {qid}: llamada inexistente {cid}")
                if calls[cid]["side"] != lado or calls[cid]["query_id"] != qid:
                    blocked(f"comparison {qid}: {cid} con lado o consulta cruzados")
                if cid in usados:
                    blocked(f"call {cid}: aparece en dos papeles o consultas")
                usados[cid] = qid
            o = calls[par["observed_call_id"]]
            r = calls[par["reference_call_id"]]
            if o["comparison_key"] != r["comparison_key"] or o["comparison_key"] != q["comparison_key"]:
                blocked(f"comparison {qid}: claves de comparación distintas")
        for cid in comp["additional_observed_call_ids"]:
            if cid not in calls or calls[cid]["side"] != "observed" or calls[cid]["query_id"] != qid:
                blocked(f"comparison {qid}: adicional {cid} inválida")
            if cid in usados:
                blocked(f"call {cid}: aparece en dos papeles o consultas")
            usados[cid] = qid
        rev = comp["review"]
        if not rev["equivalence_checked"] or not rev["required_evidence_preserved"]:
            pend(f"{qid}: revisión sin equivalencia o conservación", CAUSE_RECUPERACION)
        obs_ids = {c["id"] for c in ev["calls"] if c["query_id"] == qid and c["side"] == "observed"}
        for campo, items in (("fact_evidence", q["required_facts"]), ("reference_evidence", q["required_references"])):
            cubiertos = {x["item_id"] for x in rev[campo] if x["observed_call_id"] in obs_ids}
            faltan = [x["id"] for x in items if x["id"] not in cubiertos]
            if faltan:
                pend(f"{qid}: hechos/referencias sin evidencia {faltan}", CAUSE_RECUPERACION)

    # C-006: cada llamada aparece exactamente una vez; las no comparadas se
    # omitirían de los agregados e inflarían el ahorro.
    huerfanas = sorted(set(calls) - set(usados))
    if huerfanas:
        blocked(f"calls sin comparación: {huerfanas}")

    elegibles = [q for q in ev["queries"] if q["eligibility"] == "eligible"]
    comparadas = [q for q in elegibles if q["id"] in comps and comps[q["id"]]["paired_calls"]]
    if len(comparadas) < 3:
        pend(f"comparaciones elegibles: {len(comparadas)} < 3", CAUSE_MEDICION)
    # Contrato: una comparación por consulta elegible. Omitir una elegible
    # (p. ej. la más cara) sesgaría el ahorro aunque se alcance el mínimo.
    sin_comparar = sorted(q["id"] for q in elegibles if q["id"] not in comps)
    if sin_comparar:
        pend(f"consultas elegibles sin comparación: {sin_comparar}", CAUSE_MEDICION)

    prevista = {"unsupported": "compressed_origin", "supported": "automatic_rewrite"}.get(
        profile["rewrite_status"])
    for q in comparadas:
        comp = comps[q["id"]]
        for par in comp["paired_calls"]:
            o = calls[par["observed_call_id"]]
            if prevista is None:
                # Sin capacidad conocida no hay evidencia para culpar al agente.
                pend(f"{q['id']}: capacidad desconocida, ruta sin confirmar", CAUSE_MEDICION)
            elif o["route"] != prevista:
                # Reescritura automática en un agente que no la admite es
                # incompatibilidad; cualquier otra desviación, ausencia de uso.
                causa = CAUSE_INCOMPATIBILIDAD if o["route"] == "automatic_rewrite" else CAUSE_USO
                pend(f"{q['id']}: ruta {o['route']} != prevista {prevista}", causa)
        if q["manual_tool_coaching"]:
            pend(f"{q['id']}: hubo indicación manual de herramienta", CAUSE_USO)

    for grupo in ("surface_checks", "regression_checks"):
        for nombre, chk in ev[grupo].items():
            if not chk["passed"]:
                pend(f"{grupo}.{nombre}: no superado", CAUSE_INSTRUCCIONES)

    per_query = []
    ref_total = obs_total = 0
    for q in comparadas:
        comp = comps[q["id"]]
        rb = sum(len(calls[p["reference_call_id"]]["visible_text"].encode("utf-8")) for p in comp["paired_calls"])
        ob = sum(len(calls[p["observed_call_id"]]["visible_text"].encode("utf-8")) for p in comp["paired_calls"])
        ob += sum(len(calls[cid]["visible_text"].encode("utf-8")) for cid in comp["additional_observed_call_ids"])
        ref_total += rb
        obs_total += ob
        per_query.append({"query_id": q["id"], "reference_complete_bytes": rb,
                          "observed_complete_bytes": ob, "delta_bytes": rb - ob})
    if ref_total <= 0:
        blocked("referencia agregada no positiva")
    delta = ref_total - obs_total
    percent = 100.0 * delta / ref_total
    if delta <= 0:
        pend(f"ahorro agregado no positivo: {delta} bytes", CAUSE_MEDICION)

    pending_reasons = [r for r, _ in pending]
    causes = {c for _, c in pending}

    status = "pending" if pending_reasons else "closed"
    return {
        "report_version": REPORT_VERSION,
        "status": status,
        "agent_profile": profile,
        "session_id": session["id"],
        "manifest_digest": session["manifest_sha256"],
        "repository_revision": session["repository_revision"],
        "comparisons": per_query,
        "aggregate_reference_bytes": ref_total,
        "aggregate_observed_bytes": obs_total,
        "delta_bytes": delta,
        "saving_percent": percent,
        "surface_checks": ev["surface_checks"],
        "regression_checks": ev["regression_checks"],
        "failure_reasons": pending_reasons,
        "cause": sorted(set(causes)),
        "unit": "utf8_bytes",
        "measurement_method": CODEC,
    }


def main(argv=None):
    ap = argparse.ArgumentParser(description="Validador offline de evidencia 038")
    ap.add_argument("--input", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args(argv)
    root = Path.cwd()
    try:
        raw = Path(args.input).read_bytes()
    except OSError as exc:
        report = {"report_version": REPORT_VERSION, "status": "blocked",
                  "failure_reasons": [f"entrada ilegible: {exc}"], "cause": [CAUSE_MEDICION]}
        Path(args.output).write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        return EXIT["blocked"]
    try:
        ev = parse_bytes(raw)
        report = evaluate(ev, root)
    except Blocked as exc:
        report = {"report_version": REPORT_VERSION, "status": "blocked",
                  "failure_reasons": [exc.reason], "cause": [exc.cause]}
    except ValueError as exc:
        report = {"report_version": REPORT_VERSION, "status": "blocked",
                  "failure_reasons": [f"JSON inválido: {exc}"], "cause": [CAUSE_MEDICION]}
    Path(args.output).write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    return EXIT[report["status"]]


if __name__ == "__main__":
    sys.exit(main())
