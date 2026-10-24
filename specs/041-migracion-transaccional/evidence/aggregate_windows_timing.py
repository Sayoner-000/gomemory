"""Agrega marcas del diagnóstico sin dependencias ni aserciones de rendimiento."""
import argparse
import collections
import json
from pathlib import Path
import re
import statistics


def aggregate(timing_path, tests_path):
    mark = re.compile(r'pid=(\d+) cmd="([^"]*)" (\S+)\s+\+(\d+)ms(.*)')
    starts = {}
    spans = collections.defaultdict(list)
    active = {}
    opens = []
    pairs = {
        "db.open.end": "db.open.begin",
        "dbfile.end": "dbfile.begin",
        "dbfile.create.end": "dbfile.create.begin",
        "dbfile.chmod.end": "dbfile.chmod.begin",
        "migration.connect.end": "migration.connect.begin",
        "migration.lock.end": "migration.lock.begin",
        "migration.schema.end": "migration.schema.begin",
        "migration.commit.end": "migration.commit.begin",
        "fold.end": "fold.begin",
        "expand.exit": "expand.flush.begin",
        "toolout.end": "toolout.begin",
        "close.end": "close.begin",
    }
    for line in Path(timing_path).read_text(encoding="utf-8-sig").splitlines():
        match = mark.search(line)
        if not match:
            continue
        pid, command, label, elapsed, extra = match.groups()
        elapsed = int(elapsed)
        key = (pid, command)
        if label == "dbfile.begin":
            active[key] = {"pid": pid, "command": command, "created": False, "phases_ms": {}}
        if label == "dbfile.created" and key in active:
            active[key]["created"] = True
        if label == "migration.connect.end" and key in active:
            version = re.search(r"version (\d+)", extra)
            if version:
                active[key]["version_before"] = int(version[1])
        if label in pairs:
            started = starts.pop((*key, pairs[label]), None)
            if started is not None:
                duration = elapsed - started
                span = pairs[label].removesuffix(".begin")
                spans[(command, span)].append(duration)
                if key in active:
                    active[key]["phases_ms"][span] = duration
        if label.endswith(".begin"):
            starts[(*key, label)] = elapsed
        if label == "dbfile.end" and key in active:
            opens.append(active.pop(key))
    counts = collections.defaultdict(collections.Counter)
    for line in Path(tests_path).read_text(encoding="utf-8-sig").splitlines():
        match = re.match(r"--- (PASS|FAIL): (\S+)", line)
        if match:
            counts[match[2]][match[1]] += 1
    def stats(values):
        return {"n": len(values), "p50_ms": statistics.median(values), "max_ms": max(values)}
    groups = collections.defaultdict(list)
    for opened in opens:
        kind = "new_file" if opened["created"] else "existing_file"
        groups[kind].append(opened["phases_ms"]["dbfile"])
    return {
        "tests": dict(counts),
        "spans": [{"command": command, "span": span, **stats(values)}
                  for (command, span), values in sorted(spans.items())],
        "database_opens": {kind: stats(values) for kind, values in sorted(groups.items())},
        "slowest_opens": sorted(opens, key=lambda item: item["phases_ms"]["dbfile"], reverse=True)[:10],
        "incomplete_opens": len(active),
        "unmatched_begin_marks": len(starts),
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("timing")
    parser.add_argument("tests")
    args = parser.parse_args()
    print(json.dumps(aggregate(args.timing, args.tests), indent=2))
