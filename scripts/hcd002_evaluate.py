#!/usr/bin/env python3
"""Capture HCD-002 get_context deliveries in isolated stores (task 4).

Follows the protocol fixed in hcd/HCD-002-*.md: one explicit arm per
invocation, complete stdout of every delivery and recovery, monotonic latency
and failures kept. It observes CLI stdout only; it never claims what an agent's
model received. Fixtures are synthetic and contain no credentials.
"""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

SESSION_START = json.dumps({"hook_event_name": "SessionStart", "source": "startup"})


def sha(data):
    return hashlib.sha256(data).hexdigest()


def summarize(deliveries, duration_ns, passed, error=""):
    return {"bytes": sum(len(x.encode("utf-8")) for x in deliveries),
            "deliveries": deliveries, "duration_ns": duration_ns,
            "quality_passed": passed, "error": error,
            "visibility": "cli_stdout_only", "model_input_verified": False}


def p95(values):
    if not values:
        return None
    return sorted(values)[math.ceil(.95 * len(values)) - 1]


def visible_ok(text, literals):
    return "## HCD activo" in text and all(lit in text for lit in literals)


def memory_id(search_output, title):
    """Return the id of the row whose title matches, from `mem search`'s table.

    The table truncates titles, so a row is identified by the title's unique
    prefix (up to its first colon); checkpoints never match.
    """
    key = title.split(":")[0] + ":"
    for line in search_output.splitlines():
        match = re.match(r"^(\d+)\s+(\S+)\s+(.*)$", line)
        if match and match.group(2) != "checkpoint" and match.group(3).startswith(key):
            return match.group(1)
    return None


def run_case(binary, case, project):
    deliveries = []
    started = None
    with tempfile.TemporaryDirectory(prefix="gomemory-hcd002-case-") as temporary:
        root = Path(temporary)
        work = root / "work"
        (work / ".memory").mkdir(parents=True)
        (work / ".memory/settings.json").write_text(json.dumps({"context_compression_level": "max"}))
        (root / "home").mkdir()
        bundle = root / "bundle.json"
        bundle.write_text(json.dumps(project["bundle"], ensure_ascii=False))
        env = dict(os.environ, GOMEMORY_DATA_HOME=str(root / "store"), HOME=str(root / "home"))

        def call(args, visible=False, data=None):
            result = subprocess.run([str(binary)] + args, cwd=str(work), env=env, input=data,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
            text = result.stdout.decode("utf-8", errors="strict")
            if visible:
                deliveries.append(text)
            if result.returncode:
                raise RuntimeError("command=%s exit=%s stderr=%s" % (
                    args[0], result.returncode, result.stderr.decode("utf-8", errors="replace")[:300]))
            return text

        try:
            # Setup is outside request latency and never touches the real store.
            subprocess.run(["git", "init", "-q"], cwd=str(work), check=True)
            call(["session", "start"])
            call(["import", str(bundle)])
            call(["hcd", "start", "--kind", "technical", "--lane", "fast", "--title", "Unidad sintética activa",
                  "--why", "corpus HCD-002", "--intent", "corpus HCD-002", "--json", "--client", "claude"])
            started = time.monotonic_ns()
            category, literals = case["category"], case["required_visible"]
            if category == "arranque":
                passed = visible_ok(call(["hook", "session-start"], visible=True, data=SESSION_START.encode()), literals)
            elif category == "compactacion":
                call(["context"], visible=True)
                restored = call(["hook", "post-compact"], visible=True)
                passed = visible_ok(restored + call(["context"], visible=True), literals)
            elif category == "full":
                call(["context"], visible=True)
                passed = visible_ok(call(["context", "--full"], visible=True), literals)
            elif category == "recuperacion":
                passed = visible_ok(call(["context"], visible=True), literals)
                for target in case["recover"]:
                    found = memory_id(call(["search", target["title"]], visible=True), target["title"])
                    recovered = call(["get", found], visible=True) if found else ""
                    expected = next(m["content"] for m in project["bundle"]["memories"] if m["title"] == target["title"])
                    passed = passed and found is not None and expected in recovered and sha(expected.encode()) == target["sha256"]
            else:
                raise ValueError("unknown category")
            return summarize(deliveries, time.monotonic_ns() - started, passed)
        except (RuntimeError, ValueError, StopIteration, subprocess.TimeoutExpired,
                subprocess.CalledProcessError, UnicodeError) as error:
            duration = time.monotonic_ns() - started if started is not None else 0
            return summarize(deliveries, duration, False, str(error))


def self_test():
    result = summarize(["á", "原", "receipt", "recovery"], 123, True)
    if result["bytes"] != 20 or result["duration_ns"] != 123:
        raise AssertionError("complete UTF-8 byte accounting")
    if result["model_input_verified"] or result["visibility"] != "cli_stdout_only":
        raise AssertionError("unsupported model visibility claim")
    if p95(list(range(1, 101))) != 95:
        raise AssertionError("nearest-rank p95")
    failed = summarize(["partial"], 10, False, "timeout")
    if failed["quality_passed"] or failed["error"] != "timeout" or failed["bytes"] != 7:
        raise AssertionError("failure discarded")
    if visible_ok("## Reglas\nLIT", ["LIT"]) or not visible_ok("## HCD activo\nLIT", ["LIT"]):
        raise AssertionError("HCD activo and literals are both mandatory")
    table = ("ID Tipo Título Contenido\n-- ---- ------ ---------\n"
             "12 checkpoint proyecto decision 0007: sobre x...\n34 decision proyecto decision 0007: sobre ca...\n")
    if memory_id(table, "proyecto decision 0007: sobre canal") != "34":
        raise AssertionError("search id parsing")
    print("capture self-test passed (UTF-8 bytes, p95, failures, mandatory literals, recovery lookup, unknown model frontier)")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--arm", choices=("baseline", "improved"))
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not args.binary or not args.output or not args.arm:
        parser.error("--binary, --arm and --output required")
    binary = args.binary.resolve(strict=True)
    corpus_path = Path(__file__).resolve().parent.parent / "tests/fixtures/hcd002/corpus.json"
    corpus_bytes = corpus_path.read_bytes()
    corpus = json.loads(corpus_bytes)
    results = []
    report = {"schema": 1, "arm": args.arm, "binary_sha256": sha(binary.read_bytes()),
              "corpus_sha256": sha(corpus_bytes), "model_input_verified": False,
              "global_verdict": "inconclusive", "results": results,
              "limitations": ["CLI stdout is observed; what each agent's model receives is not.",
                              "Arms run one after another, not interleaved.",
                              "Synthetic corpus calibrated to local usage; environment sections excluded."]}
    # Persist each observation so an interruption never erases failures.
    args.output.parent.mkdir(parents=True, exist_ok=True)
    for repetition in range(5):
        for case in corpus["cases"]:
            value = run_case(binary, case, corpus["projects"][case["project"]])
            value.update(case=case["id"], repetition=repetition + 1)
            results.append(value)
            report["p95_ns"] = p95([x["duration_ns"] for x in results])
            args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        print("%s: repetition %s/5 captured" % (args.arm, repetition + 1), flush=True)


if __name__ == "__main__":
    main()
