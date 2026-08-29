#!/usr/bin/env python3
"""Ejercita el protocolo ACR completo contra el servidor MCP por stdio."""
import json, subprocess, sys

MEM, ROOT = sys.argv[1], sys.argv[2]
p = subprocess.Popen([MEM, "mcp", "--root", ROOT],
                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True, bufsize=1)
n = [0]
def rpc(method, params=None):
    n[0] += 1
    p.stdin.write(json.dumps({"jsonrpc":"2.0","id":n[0],"method":method,
                              "params":params or {}}) + "\n"); p.stdin.flush()
    while True:
        line = p.stdout.readline()
        if not line: raise SystemExit("servidor cerrado")
        msg = json.loads(line)
        if msg.get("id") == n[0]: return msg

def tool(name, args):
    r = rpc("tools/call", {"name": name, "arguments": args})
    if "error" in r: return {"__error__": r["error"]["message"]}
    txt = r["result"]["content"][0]["text"]
    try: return json.loads(txt)
    except Exception: return {"__raw__": txt}

rpc("initialize", {"protocolVersion":"2024-11-05","capabilities":{},
                   "clientInfo":{"name":"acr-smoke","version":"1"}})
p.stdin.write(json.dumps({"jsonrpc":"2.0","method":"notifications/initialized"})+"\n"); p.stdin.flush()

def paso(titulo, res):
    print(f"  ▸ {titulo}")
    print("    " + json.dumps(res, ensure_ascii=False)[:220])
    return res

r = paso("review_start", tool("review_start", {
    "target_type":"diff","revision":"HEAD","digest":"sha256:v0","scope":["store.go"]}))
rid = r["review"]["ID"] if "review" in r else r.get("review_id")

ids = {}
for who, local in (("A","A-001"), ("B","B-004")):
    res = paso(f"review_submit ({who})", tool("review_submit", {
        "review_id":rid,"reviewer":who,"target_digest":"sha256:v0","status":"success",
        "findings":[{"local_id":local,"location":"store.go:2","severity":"HIGH",
                     "category":"concurrency","claim":"escritura concurrente pisa el estado",
                     "evidence_class":"deterministic","evidence":["el mapa no está protegido"],
                     "confidence":"high"}]}))
    ids[who] = res["finding_ids"][local]   # los IDs los asigna gomemory, no el llamador

c = paso("review_consensus", tool("review_consensus", {
    "review_id":rid,
    "matches":[{"status":"CONFIRMED","finding_id_a":ids["A"],"finding_id_b":ids["B"],
                "severity":"HIGH","claim":"escritura concurrente pisa el estado"}]}))

paso("review_fix_record", tool("review_fix_record", {
    "review_id":rid,"addressed_consensus_ids":["C-001"],
    "base_target_digest":"sha256:v0","fixed_target_digest":"sha256:v1",
    "modified_paths":["store.go"],"verification":["go test ./..."]}))

for who in ("A","B"):
    tool("review_submit", {"review_id":rid,"reviewer":who,"target_digest":"sha256:v0",
                           "status":"success","findings":[]})
paso("review_rejudge", tool("review_rejudge", {
    "review_id":rid,"states":{"C-001":"RESOLVED"}}))

paso("review_finalize", tool("review_finalize", {"review_id":rid}))
paso("review_promote_memory", tool("review_promote_memory", {
    "review_id":rid,"learnings":{"C-001":{
        "category":"concurrency","component":"store",
        "problem":"escrituras concurrentes pisaban el estado",
        "root_cause":"el mapa no estaba protegido por mutex",
        "resolution":"se añadió sync.RWMutex alrededor del acceso",
        "verification":["go test -race ./..."],"confidence":"high"}}}))

print("\n  ▸ RECHAZOS (las invariantes, no el prompt):")
for nombre, args in (
    ("corregir un SUSPECT", {"review_id":rid,"addressed_consensus_ids":["S-999"],
      "base_target_digest":"sha256:v1","fixed_target_digest":"sha256:v2"}),
):
    e = tool("review_fix_record", args)
    print(f"    {nombre}: {e.get("__raw__", e.get("__error__","NO RECHAZADO ❌"))[:110]}")
p.stdin.close(); p.wait(timeout=10)
