#!/usr/bin/env python3
"""Generate the HCD-002 synthetic corpus deterministically.

The corpus imitates the shape of real get_context usage (task 1 profile):
memory counts by type, content lengths, a pinned work-rules memory and a long
tail project. It never reads real memories and contains no credentials.
Run: python3 scripts/hcd002_corpus.py > tests/fixtures/hcd002/corpus.json
"""
import hashlib
import json
import math
import random
import sys
from datetime import datetime, timedelta, timezone

SEED = 20261029
# Curated mix from task 1 (checkpoints excluded), in percent.
CURATED_MIX = {"decision": 27.7, "bugfix": 19.0, "discovery": 18.6, "architecture": 9.9,
               "preference": 9.5, "pattern": 8.9, "learning": 6.3}
# Lognormal fitted to content length median 986 and p95 3762 characters.
LEN_MU, LEN_SIGMA = math.log(986), (math.log(3762) - math.log(986)) / 1.645
WORDS = ("contrato capa puerto adaptador usecase dominio presupuesto entrega sesión delta "
         "compactación recuperación huella referencia índice memoria decisión prueba rojo verde "
         "migración esquema transacción repositorio servidor cliente hook canal arranque plan "
         "tarea evidencia revisión aprobación norma constitución playbook motor documento slice "
         "latencia tamaño corpus protocolo observación captura brazo umbral calidad literal "
         "aislamiento proyecto privacidad secreto redacción original omisión marcador lote "
         "búsqueda selección prioridad crítico relevante opcional duplicado sinapsis conflicto").split()
RULES_LITERAL = "REGLA-HCD002-FIJADA"


def sha(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def prose(rng, length, tag):
    words = [tag]
    while sum(len(w) + 1 for w in words) < length:
        words.append(rng.choice(WORDS))
    return " ".join(words)[:length]


def project(rng, name, curated, relations, long_tail=False):
    start = datetime(2026, 8, 1, tzinfo=timezone.utc)
    kinds = []
    for kind, pct in CURATED_MIX.items():
        kinds += [kind] * round(curated * pct / 100)
    kinds = kinds[:curated] + ["decision"] * max(0, curated - len(kinds))
    rng.shuffle(kinds)
    memories = [{"ref_id": 1, "type": "preference", "title": "Reglas de trabajo del proyecto",
                 "content": RULES_LITERAL + ": " + prose(rng, 1180, "reglas"),
                 "topic_key": "gomemory:work-rules",
                 "created_at": start.isoformat(), "updated_at": start.isoformat()}]
    for i, kind in enumerate(kinds, start=2):
        when = start + timedelta(minutes=97 * i)
        length = int(rng.lognormvariate(LEN_MU, LEN_SIGMA) * (3 if long_tail else 1))
        # Real titles read like a sentence (~40-90 chars), not like an id.
        title = "%s %s %04d: %s" % (name, kind, i, prose(rng, rng.randint(25, 70), "sobre"))
        memories.append({"ref_id": i, "type": kind, "title": title,
                         "content": prose(rng, max(80, min(length, 20000)), "%s-%04d" % (name.upper(), i)),
                         "created_at": when.isoformat(), "updated_at": when.isoformat()})
    # Checkpoints: half of all stored memories in real usage (task 1).
    for j in range(curated):
        i = len(memories) + 1
        when = start + timedelta(minutes=97 * i)
        memories.append({"ref_id": i, "type": "checkpoint", "title": "checkpoint %s %04d" % (name, j),
                         "content": prose(rng, 160, "CHECKPOINT-%04d" % j),
                         "created_at": when.isoformat(), "updated_at": when.isoformat()})
    # Real projects interleave checkpoints with curated memories over time, and
    # get_context only reads the most recent ones: dates are shuffled together.
    slots = list(range(2, len(memories) + 1))
    rng.shuffle(slots)
    for m, slot in zip(memories[1:], slots):
        m["created_at"] = m["updated_at"] = (start + timedelta(minutes=97 * slot)).isoformat()
    # Real projects of this size carry ~500 synapses, almost all "related".
    curated_ids = [m["ref_id"] for m in memories if m["type"] != "checkpoint"]
    pairs = set()
    while len(pairs) < min(relations, len(curated_ids) * (len(curated_ids) - 1) // 2):
        a, b = sorted(rng.sample(curated_ids, 2))
        pairs.add((a, b))
    rels = [{"ref_a": a, "ref_b": b, "relation": "related", "confidence": 0.8, "created_at": start.isoformat()} for a, b in sorted(pairs)]
    bundle = {"version": 2, "exported_at": start.isoformat(), "source_project": name,
              "memories": memories, "relations": rels}
    canonical = json.dumps(bundle, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return {"bundle": bundle, "sha256": sha(canonical)}


def cases(projects):
    out = []
    rep = projects["representativo"]["bundle"]["memories"]
    tail = projects["cola"]["bundle"]["memories"]
    decisions = [m for m in rep if m["type"] == "decision"]
    tail_decisions = [m for m in tail if m["type"] == "decision"]
    recent = sorted(decisions, key=lambda m: m["created_at"])[-3:]
    old = sorted(decisions, key=lambda m: m["created_at"])[:4]
    for category in ("arranque", "compactacion", "full", "recuperacion"):
        for n in range(1, 6):
            use_tail = n == 5
            pool = tail_decisions if use_tail else decisions
            visible = [RULES_LITERAL] + [m["title"] for m in (sorted(pool, key=lambda m: m["created_at"])[-3:] if use_tail else recent)]
            case = {"id": "%s-%02d" % (category, n), "category": category,
                    "project": "cola" if use_tail else "representativo",
                    "required_visible": visible, "recover": [], "full": category == "full"}
            if category == "recuperacion":
                target = sorted(pool, key=lambda m: m["created_at"])[0] if use_tail else old[n - 1]
                case["recover"] = [{"title": target["title"], "sha256": sha(target["content"])}]
            out.append(case)
    return out


def main():
    rng = random.Random(SEED)
    projects = {"representativo": project(rng, "representativo", 376, 503),
                "cola": project(rng, "cola", 60, 40, long_tail=True)}
    corpus = {"schema": 1, "seed": SEED, "generator": "scripts/hcd002_corpus.py",
              "profile": {"curated_target": 376, "curated_mix_pct": CURATED_MIX,
                          "content_chars_median": 986, "content_chars_p95": 3762, "relations_target": 503,
                          "emitted_tokens_median": 4834, "emitted_tokens_p95": 5920},
              "projects": projects, "cases": cases(projects)}
    # Bytes, not text: in Windows text mode turns "\n" into "\r\n" and encodes
    # with the console code page, and the fixture is compared byte for byte.
    sys.stdout.buffer.write((json.dumps(corpus, ensure_ascii=False, indent=1) + "\n").encode("utf-8"))


if __name__ == "__main__":
    main()
