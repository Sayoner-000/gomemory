#!/usr/bin/env python3
"""Capture real CLI deliveries in isolated stores; never claim model visibility.

Only one explicitly selected arm is run per invocation. Fixtures contain no
credentials. Requests, complete stdout and errors remain in the local report.
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


def run_case(binary, case, sources):
    deliveries = []
    started = None
    with tempfile.TemporaryDirectory(prefix="gomemory-hcd001-case-") as temporary:
        root = Path(temporary)
        work = root / "project"
        work.mkdir()
        (work / ".memory").mkdir()
        (work / ".memory/settings.json").write_text(json.dumps({"context_compression_level": "max"}))
        env = dict(os.environ, GOMEMORY_DATA_HOME=str(root / "store"))

        def call(args, visible=False, data=None):
            result = subprocess.run([str(binary)] + args, cwd=str(work), env=env,
                                    input=data, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=30)
            text = result.stdout.decode("utf-8", errors="strict")
            if visible:
                deliveries.append(text)
            if result.returncode:
                raise RuntimeError("command=%s exit=%s stderr=%s" %
                                   (args[0], result.returncode, result.stderr.decode("utf-8", errors="replace")))
            return text

        try:
            # Setup is outside request latency, and never touches the real store.
            call(["session", "start"])
            for source in sources:
                if sha(source["content"].encode()) != source["sha256"]:
                    raise ValueError("source integrity mismatch")
                call(["save", "-t", source["title"], "-y", source["type"], source["content"]])
            started = time.monotonic_ns()
            category = case["category"]
            source = next((x for x in sources if x["id"] == case["expected_source"]), None)
            if category in ("search", "no_match"):
                output = call(["pack", "build", "--task", case["query"], "--max-tokens", "4000",
                               "--no-speckit", "--no-code-graph"], visible=True)
                if category == "search":
                    passed = case["required_literal"] in output
                else:
                    # Read-only extra capture is counted, never hidden as free.
                    structured = json.loads(call(["pack", "build", "--task", case["query"],
                                                 "--max-tokens", "4000", "--no-speckit",
                                                 "--no-code-graph", "--json"], visible=True))
                    passed = len(structured.get("Items") or []) == 0
            elif category == "recovery":
                result = json.loads(call(["pack", "compress", "--level", "max", "--json"],
                                         visible=True, data=source["content"].encode()))
                refs = result.get("refs", result.get("Refs", []))
                if refs:
                    recovered = [call(["pack", "retrieve", ref], visible=True) for ref in refs]
                    passed = len(recovered) == 1 and sha(recovered[0].encode()) == source["sha256"]
                else:
                    # No omission: identity must hold; this does not prove recovery.
                    content = result.get("content", result.get("Content", ""))
                    passed = content == source["content"]
            elif category == "compaction":
                call(["context"], visible=True)
                restored = call(["hook", "post-compact"], visible=True)
                output = call(["context"] + (["--full"] if case["full"] else []), visible=True)
                passed = case["required_literal"] in restored + output
            else:
                raise ValueError("unknown category")
            return summarize(deliveries, time.monotonic_ns() - started, passed)
        except (RuntimeError, ValueError, subprocess.TimeoutExpired, UnicodeError) as error:
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
    print("capture self-test passed (complete deliveries, recoveries, p95, failures, unknown model frontier)")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--arm", choices=("baseline", "go"))
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not args.binary or not args.output or not args.arm:
        parser.error("--binary, --arm and --output required")
    binary = args.binary.resolve(strict=True)
    corpus_path = Path(__file__).resolve().parent.parent / "tests/fixtures/hcd001/corpus.json"
    corpus_bytes = corpus_path.read_bytes()
    corpus = json.loads(corpus_bytes)
    results = []
    report = {"schema": 1, "arm": args.arm, "binary_sha256": sha(binary.read_bytes()),
              "corpus_sha256": sha(corpus_bytes), "model_input_verified": False,
              "global_verdict": "inconclusive", "results": results,
              "limitations": ["CLI output is observed, Codex model input is not.",
                              "Arms run separately: inter-arm order rotation must be checked by the caller.",
                              "Synthetic contract corpus; not general coding-task accuracy."]}
    # Persist each observation so interruption never erases failures.
    args.output.parent.mkdir(parents=True, exist_ok=True)
    for repetition in range(5):
        for case in corpus["cases"]:
            value = run_case(binary, case, corpus["sources"])
            value.update(case=case["id"], repetition=repetition + 1)
            results.append(value)
            report["p95_ns"] = p95([x["duration_ns"] for x in results])
            args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        print("%s: repetition %s/5 captured" % (args.arm, repetition + 1), flush=True)


if __name__ == "__main__":
    main()
