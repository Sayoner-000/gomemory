#!/usr/bin/env python3
"""Evaluate complete HCD014 captures; wire bytes never imply billed tokens.

Inputs are evidence, not trusted attestations: their origin still needs manual
review. Self-tests use synthetic data internally and never emit accepted runs.
"""
import argparse
import copy
import hashlib
import json
import math
from pathlib import Path
import sys
import unittest


ROOT = Path(__file__).resolve().parents[1]
CORPUS = ROOT / "tests/fixtures/hcd014/corpus.json"
AGENTS = ("codex", "opencode", "claude")
NORMALIZATIONS = ("none", "strip_trailing_newlines")
ACCOUNTING = ("selection_removed_bytes", "compression_removed_bytes",
              "delta_removed_bytes", "instruction_added_bytes",
              "metadata_added_bytes", "recovery_added_bytes")


class InvalidEvidence(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise InvalidEvidence(message)


def digest(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def reject_constant(value):
    raise InvalidEvidence("non-finite JSON value: " + value)


def load(path):
    return json.loads(path.read_text(encoding="utf-8"), parse_constant=reject_constant)


def nonnegative(value, name, integer=False):
    require(type(value) in ((int,) if integer else (int, float)), name + " must be numeric")
    require(math.isfinite(value) and value >= 0, name + " must be finite and nonnegative")
    return value


def run_metrics(run, expected):
    require(isinstance(run, dict), "missing run")
    require(run.get("result") == expected, "task result differs from the exact oracle")
    latency = nonnegative(run.get("latency_ms"), "latency_ms")
    requests = run.get("requests")
    require(isinstance(requests, list) and requests, "zero captured requests")
    total = recovery = 0
    bodies = []
    usages = []
    for index, request in enumerate(requests):
        require(isinstance(request, dict), "invalid request record")
        require(request.get("sequence") == index, "request sequence is incomplete")
        require(request.get("visibility") == "provider_request", "local stdout/host callback is not a provider request")
        require(request.get("origin") == "real_agent_real_provider", "synthetic providers do not prove quality or session savings")
        require(request.get("complete") is True, "truncated or incomplete request capture")
        body = request.get("body")
        require(isinstance(body, str) and body, "missing literal request body")
        require(request.get("sha256") == digest(body), "request body hash mismatch")
        parsed = json.loads(body, parse_constant=reject_constant)
        require(isinstance(parsed, dict), "provider body must be a JSON object")
        require(request.get("phase") in ("ordinary", "recovery"), "unknown request phase")
        size = len(body.encode("utf-8"))
        total += size
        if request["phase"] == "recovery":
            recovery += size
        bodies.append(parsed)
        # Preserve provider observations as observations. Do not estimate them
        # from bytes or sum overlapping cached/input counters into billed cost.
        if "provider_usage" in request:
            usage = request["provider_usage"]
            require(isinstance(usage, dict) and usage, "invalid provider usage")
            for key, value in usage.items():
                nonnegative(value, "provider_usage." + key, integer=True)
            usages.append(usage)
    return {"bytes": total, "recovery_bytes": recovery, "latency_ms": latency,
            "requests": len(requests), "provider_usage_observed": usages}, bodies


def strings_in(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, list):
        for item in value:
            yield from strings_in(item)
    elif isinstance(value, dict):
        for item in value.values():
            yield from strings_in(item)


def evaluate_agent(data, corpus, corpus_hash, agent):
    require(isinstance(data, dict) and data.get("schema_version") == 1, "invalid capture schema")
    require(data.get("agent") == agent, "agent does not match input filename")
    for key in ("version", "mode", "route", "captured_at", "source_artifact"):
        require(isinstance(data.get(key), str) and data[key].strip(), "missing " + key)
    require(data.get("corpus_sha256") == corpus_hash, "corpus revision mismatch")
    normalization = data.get("host_output_normalization", "none")
    require(normalization in NORMALIZATIONS, "unknown host output normalization")
    cases = data.get("cases")
    require(isinstance(cases, list) and cases, "zero comparison cases")
    by_id = {}
    for case in cases:
        require(isinstance(case, dict) and isinstance(case.get("id"), str), "invalid case")
        require(case["id"] not in by_id, "duplicate case")
        by_id[case["id"]] = case
    require(set(by_id) == {c["id"] for c in corpus["cases"]}, "missing or foreign corpus cases")
    output = []
    totals = {"baseline_bytes": 0, "automatic_bytes": 0,
              "baseline_latency_ms": 0, "automatic_latency_ms": 0}
    stages = dict.fromkeys(ACCOUNTING, 0)
    for fixture in corpus["cases"]:
        case = by_id[fixture["id"]]
        # A host may drop final newlines before any hook sees the output; only
        # that declared change is forgiven, and every other byte stays frozen.
        frozen = fixture["output"].rstrip("\n") if normalization != "none" else fixture["output"]
        frozen_sha256 = digest(frozen)
        before, baseline_bodies = run_metrics(case.get("baseline"), fixture["expected_result"])
        after, automatic_bodies = run_metrics(case.get("automatic"), fixture["expected_result"])
        require(frozen in set(strings_in(baseline_bodies)), "baseline never consumed the frozen tool output")
        accounting = case.get("accounting")
        require(isinstance(accounting, dict) and set(accounting) == set(ACCOUNTING), "missing stage accounting")
        for name in ACCOUNTING:
            nonnegative(accounting[name], name, integer=True)
            stages[name] += accounting[name]
        net = before["bytes"] - after["bytes"]
        removed = sum(accounting[n] for n in ACCOUNTING[:3])
        added = sum(accounting[n] for n in ACCOUNTING[3:])
        require(removed - added == net, "stage accounting does not reconcile with complete wire bytes")
        require(after["recovery_bytes"] - before["recovery_bytes"] == accounting["recovery_added_bytes"], "recovery overhead omitted or double-counted")
        recoveries = case.get("recoveries", [])
        require(isinstance(recoveries, list), "invalid recoveries")
        if fixture["requires_recovery"]:
            require(recoveries and after["recovery_bytes"] > 0, "missing observed recovery")
        for recovery in recoveries:
            require(isinstance(recovery, dict), "invalid recovery record")
            content = recovery.get("recovered_content")
            require(isinstance(content, str) and digest(content) == frozen_sha256, "recovered content hash mismatch")
            require(content in set(strings_in(automatic_bodies)), "recovered original was not consumed in the automatic run")
            require(recovery.get("expected_sha256") == frozen_sha256 and
                    recovery.get("actual_sha256") == frozen_sha256, "recovery differs from frozen original")
        if fixture["requires_compaction"]:
            require(case.get("baseline_compaction_observed") is True and
                    case.get("automatic_compaction_observed") is True, "missing host compaction observation")
        totals["baseline_bytes"] += before["bytes"]
        totals["automatic_bytes"] += after["bytes"]
        totals["baseline_latency_ms"] += before["latency_ms"]
        totals["automatic_latency_ms"] += after["latency_ms"]
        output.append({"id": fixture["id"], "net_saved_bytes": net,
                       "latency_overhead_ms": after["latency_ms"] - before["latency_ms"],
                       "baseline": before, "automatic": after})
    totals["net_saved_bytes"] = totals["baseline_bytes"] - totals["automatic_bytes"]
    require(totals["net_saved_bytes"] > 0, "no positive net savings for " + agent)
    totals["latency_overhead_ms"] = totals["automatic_latency_ms"] - totals["baseline_latency_ms"]
    return {"agent": agent, "version": data["version"], "mode": data["mode"],
            "route": data["route"], "coverage": "observed corpus requests only",
            "host_output_normalization": normalization,
            "totals": totals, "stage_attribution_reported": stages, "cases": output}


def evaluate(directory):
    raw = CORPUS.read_bytes()
    corpus = json.loads(raw)
    require(corpus.get("schema_version") == 1 and corpus.get("cases"), "empty/invalid corpus")
    corpus_hash = hashlib.sha256(raw).hexdigest()
    results, errors = [], []
    for agent in AGENTS:
        try:
            results.append(evaluate_agent(load(directory / (agent + "-comparison.json")), corpus, corpus_hash, agent))
        except (InvalidEvidence, OSError, ValueError, TypeError, KeyError) as error:
            errors.append({"agent": agent, "reason": str(error)})
    return {"schema_version": 1, "status": "passed" if not errors else "insufficient_or_failed_evidence",
            "counter": "UTF-8 bytes of every complete provider request; not billed tokens",
            "corpus_sha256": corpus_hash, "agents": results, "errors": errors,
            "limitation": "Capture provenance and stage attribution require manual review; no extrapolation beyond the corpus."}


def self_test():
    corpus = load(CORPUS)
    corpus_hash = hashlib.sha256(CORPUS.read_bytes()).hexdigest()

    def request(text, sequence=0, phase="ordinary"):
        body = json.dumps({"messages": [{"role": "tool", "content": text}]})
        return dict(sequence=sequence, phase=phase, body=body, sha256=digest(body),
                    origin="real_agent_real_provider", visibility="provider_request", complete=True)

    def capture():
        data = dict(schema_version=1, agent="codex", version="self-test", mode="test",
                    route="internal_self_test_only", captured_at="self-test",
                    source_artifact="internal test fixture; never emitted", corpus_sha256=corpus_hash, cases=[])
        for fixture in corpus["cases"]:
            baseline = dict(result=fixture["expected_result"], latency_ms=10, requests=[request(fixture["output"])])
            automatic = dict(result=fixture["expected_result"], latency_ms=12, requests=[request(fixture["expected_result"])])
            recoveries = []
            if fixture["requires_recovery"]:
                automatic["requests"].append(request(fixture["output"], 1, "recovery"))
                recoveries.append(dict(expected_sha256=fixture["output_sha256"], actual_sha256=fixture["output_sha256"], recovered_content=fixture["output"]))
            before, _ = run_metrics(baseline, fixture["expected_result"])
            after, _ = run_metrics(automatic, fixture["expected_result"])
            recovery = after["recovery_bytes"]
            net = before["bytes"] - after["bytes"]
            accounting = dict.fromkeys(ACCOUNTING, 0)
            accounting["compression_removed_bytes"] = net + recovery
            accounting["recovery_added_bytes"] = recovery
            data["cases"].append(dict(id=fixture["id"], baseline=baseline, automatic=automatic,
                                      accounting=accounting, recoveries=recoveries,
                                      baseline_compaction_observed=True, automatic_compaction_observed=True))
        return data

    def host_stripped_capture():
        """Same runs as capture(), as a host that drops final newlines shows them."""
        data = capture()
        by_id = {c["id"]: c for c in corpus["cases"]}
        for case in data["cases"]:
            text = by_id[case["id"]]["output"]
            stripped = text.rstrip("\n")
            for run in (case["baseline"], case["automatic"]):
                for item in run["requests"]:
                    item["body"] = item["body"].replace(json.dumps(text)[1:-1], json.dumps(stripped)[1:-1])
                    item["sha256"] = digest(item["body"])
            for recovery in case["recoveries"]:
                recovery.update(expected_sha256=digest(stripped), actual_sha256=digest(stripped), recovered_content=stripped)
            before, _ = run_metrics(case["baseline"], by_id[case["id"]]["expected_result"])
            after, _ = run_metrics(case["automatic"], by_id[case["id"]]["expected_result"])
            case["accounting"] = dict.fromkeys(ACCOUNTING, 0)
            case["accounting"]["compression_removed_bytes"] = before["bytes"] - after["bytes"] + after["recovery_bytes"]
            case["accounting"]["recovery_added_bytes"] = after["recovery_bytes"]
        return data

    class EvidenceChecks(unittest.TestCase):
        def check_bad(self, change):
            data = capture()
            change(data)
            with self.assertRaises(InvalidEvidence):
                evaluate_agent(data, corpus, corpus_hash, "codex")

        def test_complete(self):
            result = evaluate_agent(capture(), corpus, corpus_hash, "codex")
            self.assertGreater(result["totals"]["net_saved_bytes"], 0)
            self.assertEqual(result["totals"]["latency_overhead_ms"], 12)

        def test_zero_cases(self): self.check_bad(lambda d: d.update(cases=[]))
        def test_missing_case(self): self.check_bad(lambda d: d["cases"].pop())
        def test_wrong_oracle(self): self.check_bad(lambda d: d["cases"][0]["automatic"].update(result="almost correct"))
        def test_hash(self): self.check_bad(lambda d: d["cases"][0]["automatic"]["requests"][0].update(sha256="0" * 64))
        def test_stdout(self): self.check_bad(lambda d: d["cases"][0]["automatic"]["requests"][0].update(visibility="cli_stdout"))
        def test_stub(self): self.check_bad(lambda d: d["cases"][0]["automatic"]["requests"][0].update(origin="synthetic_provider"))
        def test_incomplete(self): self.check_bad(lambda d: d["cases"][0]["automatic"]["requests"][0].update(complete=False))
        def test_overhead(self): self.check_bad(lambda d: d["cases"][0]["accounting"].update(metadata_added_bytes=1))
        def test_recovery(self): self.check_bad(lambda d: d["cases"][3]["recoveries"][0].update(actual_sha256="0" * 64))
        def test_forged_recovery_content(self): self.check_bad(lambda d: d["cases"][3]["recoveries"][0].update(recovered_content="forged"))
        def test_compaction(self): self.check_bad(lambda d: d["cases"][4].update(automatic_compaction_observed=False))
        def test_no_gain(self):
            def change(data):
                for case in data["cases"]:
                    case["automatic"] = copy.deepcopy(case["baseline"])
                    case["accounting"] = dict.fromkeys(ACCOUNTING, 0)
                    if case["recoveries"]:
                        case["automatic"]["requests"].append(request(case["recoveries"][0]["recovered_content"], 1, "recovery"))
                        size = len(case["automatic"]["requests"][-1]["body"].encode())
                        case["accounting"]["recovery_added_bytes"] = size
            self.check_bad(change)
        def test_nonfinite(self): self.check_bad(lambda d: d["cases"][0]["automatic"].update(latency_ms=float("nan")))
        def test_unknown_normalization(self): self.check_bad(lambda d: d.update(host_output_normalization="strip_whitespace"))

        def test_declared_host_normalization(self):
            data = host_stripped_capture()
            data["host_output_normalization"] = "strip_trailing_newlines"
            self.assertEqual(evaluate_agent(data, corpus, corpus_hash, "codex")["host_output_normalization"], "strip_trailing_newlines")

        def test_undeclared_host_normalization(self):
            data = host_stripped_capture()
            with self.assertRaises(InvalidEvidence):
                evaluate_agent(data, corpus, corpus_hash, "codex")

        def test_normalization_allows_only_final_newlines(self):
            data = host_stripped_capture()
            data["host_output_normalization"] = "strip_trailing_newlines"
            recovery = data["cases"][3]["recoveries"][0]
            recovery["recovered_content"] = recovery["recovered_content"][:-1]
            with self.assertRaises(InvalidEvidence):
                evaluate_agent(data, corpus, corpus_hash, "codex")

    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(EvidenceChecks))
    return 0 if result.wasSuccessful() and result.testsRun >= 14 else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--input-dir", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.self_test:
        return self_test()
    if args.input_dir is None:
        parser.error("--input-dir is required")
    report = evaluate(args.input_dir)
    rendered = json.dumps(report, ensure_ascii=False, indent=2, allow_nan=False) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    sys.exit(main())
