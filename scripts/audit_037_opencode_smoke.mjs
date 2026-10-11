#!/usr/bin/env node
// Runner de validación de la feature 037 contra un host OpenCode v2 REAL
// (H-05/H-06). Complementa la suite simulada (gomemory.test.mjs); no la
// sustituye ni la repite.
//
// Uso:
//   node scripts/audit_037_opencode_smoke.mjs --mem-bin <mem> --opencode-bin <opencode-v2> --out-dir <dir> [--model-ready]
//
// Códigos de salida (mismos que audit_037_smoke.py):
//   0  todos los pasos seleccionados se ejecutaron y aprobaron
//   1  algún paso reprodujo un defecto o falló
//   2  preparación inválida, runtime incompatible, host no v2 o evidencia
//      pendiente (status=pending_verification); nunca se omite en silencio
//
// Aislamiento: HOME, XDG_* y GOMEMORY_DATA_HOME temporales; un proyecto git
// propio; PATH mínimo con el mem de prueba primero. No actualiza, reinicia ni
// detiene el OpenCode habitual del operador: el servicio que se arranca es el
// del HOME aislado y se detiene al terminar.
//
// --model-ready declara que el operador configuró credenciales de prueba en
// ese HOME aislado (no se copian del perfil habitual). Sin él, los pasos que
// necesitan un turno real del modelo quedan como pending_verification.

import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, readdirSync, readFileSync, existsSync, copyFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve, dirname } from "node:path";

const MIN_NODE = [22, 20, 0];
const results = [];
const evidence = { runner: "audit_037_opencode_smoke", status: "running", steps: results };

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--model-ready") out.modelReady = true;
    else if (a.startsWith("--")) out[a.slice(2)] = argv[++i];
  }
  return out;
}

function finish(code, status, outDir) {
  evidence.status = status;
  evidence.cases_run = results.length;
  if (outDir) {
    mkdirSync(outDir, { recursive: true });
    writeFileSync(join(outDir, "result.json"), JSON.stringify(evidence, null, 2) + "\n");
  }
  for (const r of results) console.log(`${r.ok === true ? "ok  " : r.ok === false ? "FAIL" : "PEND"} ${r.id} ${r.detail ?? ""}`.trimEnd());
  console.log(`estado: ${status}`);
  process.exit(code);
}

function step(id, ok, detail = "") {
  results.push({ id, ok, detail });
  return ok;
}

const args = parseArgs(process.argv.slice(2));
// olderThan compara versiones numéricas [mayor, menor, parche].
const olderThan = (have, need) => {
  for (let i = 0; i < need.length; i++) {
    if ((have[i] ?? 0) !== need[i]) return (have[i] ?? 0) < need[i];
  }
  return false;
};
if (olderThan(process.versions.node.split(".").map(Number), MIN_NODE)) {
  console.error(`RUNTIME_UNSUPPORTED: Node ${process.versions.node} < ${MIN_NODE.join(".")}`);
  process.exit(2);
}
for (const k of ["mem-bin", "opencode-bin", "out-dir"]) {
  if (!args[k]) {
    console.error(`preparación inválida: falta --${k}`);
    process.exit(2);
  }
}
const outDir = resolve(args["out-dir"]);
const memBin = resolve(args["mem-bin"]);
const ocBin = resolve(args["opencode-bin"]);
if (!existsSync(memBin) || !existsSync(ocBin)) {
  step("prepare.binaries", false, "mem u opencode no existen");
  finish(2, "preparation_failed", outDir);
}

// Banco aislado.
const bench = mkdtempSync(join(tmpdir(), "audit037-opencode-"));
const home = join(bench, "home");
const binDir = join(bench, "bin");
const project = join(bench, "project");
for (const d of [home, binDir, project]) mkdirSync(d, { recursive: true });
copyFileSync(memBin, join(binDir, "mem"));
chmodSync(join(binDir, "mem"), 0o755);
const env = {
  PATH: `${binDir}:${dirname(ocBin)}:/usr/bin:/bin:/usr/sbin:/sbin`,
  HOME: home,
  XDG_CONFIG_HOME: join(home, ".config"),
  XDG_DATA_HOME: join(home, ".local", "share"),
  XDG_STATE_HOME: join(home, ".local", "state"),
  XDG_CACHE_HOME: join(home, ".cache"),
  GOMEMORY_DATA_HOME: join(bench, "gomemory-data"),
  GOMEMORY_NO_UPDATE_CHECK: "1",
  NO_COLOR: "1",
};
evidence.bench = "temporal (eliminable)";

const run = (bin, argv, opts = {}) => spawnSync(bin, argv, { cwd: project, env, encoding: "utf8", timeout: 120_000, ...opts });

spawnSync("git", ["init", "-q"], { cwd: project, env });

// Versión del host: exige mayor 2 (la salida v2 es "opencode v2.x.y").
const v = run(ocBin, ["--version"]);
const ver = (v.stdout || v.stderr || "").trim();
evidence.opencode_version = ver;
const major = Number((ver.match(/(\d+)\.\d+\.\d+/) || [])[1]);
if (major !== 2) {
  step("host.version", null, `se requiere OpenCode 2.x; el binario reporta "${ver}"`);
  evidence.next = "Prepara un OpenCode 2.x en un banco separado (p. ej. @opencode/cli@2.0.16 con --prefix temporal) y vuelve a ejecutar.";
  finish(2, "pending_verification", outDir);
}
step("host.version", true, ver);

// Cablear gomemory en el HOME aislado y activar la compresión de salidas.
const setup = run(join(binDir, "mem"), ["setup-mcp", "--scope", "global", "--agents", "opencode"]);
step("setup.plugin", setup.status === 0, setup.status === 0 ? "" : (setup.stderr || "").slice(-200));
const settings = run(join(binDir, "mem"), ["settings", "--tool-output-compression=true", "--compression-level=max"]);
step("setup.compression", settings.status === 0, settings.status === 0 ? "" : (settings.stderr || "").slice(-200));

// Carga del plugin dual en v2 (procedimiento verificado en la feature 032).
const reload = run(ocBin, ["reload"]);
const list = run(ocBin, ["plugin", "list"]);
step("host.plugin_loaded", reload.status === 0 && /gomemory/.test(list.stdout || ""), (list.stdout || list.stderr || "").trim().slice(-200));
const logDir = join(home, ".local", "share", "opencode", "log");
const loadErrors = existsSync(logDir)
  ? readdirSync(logDir).flatMap((f) => readFileSync(join(logDir, f), "utf8").split("\n")).filter((l) => /failed to load plugin/.test(l) && /gomemory/.test(l))
  : [];
step("host.no_load_errors", loadErrors.length === 0, `${loadErrors.length} errores de carga de gomemory`);

// Pasos que requieren un turno real del modelo: consumo de la salida reducida
// (H-05) y cierre con un evento en curso (H-06) observados en el host.
if (!args.modelReady) {
  step("host.consumer_receives_reduced", null, "requiere --model-ready con credenciales de prueba en el HOME aislado");
  step("host.close_with_event_in_flight", null, "requiere --model-ready");
} else {
  const big = "for i in $(seq 1 3000); do echo \"linea $i del registro de prueba de la feature 037\"; done";
  const turn = run(ocBin, ["run", `Ejecuta exactamente este comando de shell y después responde solo OK: ${big}`]);
  step("host.turn_completed", turn.status === 0, (turn.stderr || "").slice(-200));
  // Efecto observable en gomemory: una reducción registrada por el canal de
  // opencode con ganancia. La comprobación del texto recibido por el
  // consumidor se registra a partir de la exportación de la sesión del host.
  const savings = run(join(binDir, "mem"), ["pack", "savings", "--json"]);
  const reduced = /"tool_output"|opencode/.test(savings.stdout || "");
  step("host.reduction_recorded", reduced, reduced ? "" : "sin reducción registrada para la salida de herramienta");
  step("host.consumer_receives_reduced", null, "verificación manual: exporta la sesión del host y confirma que el resultado de la herramienta contiene ⟦mem⟧ ref=…");
  const t0 = Date.now();
  const stop = run(ocBin, ["service", "stop"]);
  step("host.close_within_budget", stop.status === 0 && Date.now() - t0 <= 5000, `cierre en ${Date.now() - t0} ms`);
}

run(ocBin, ["service", "stop"]);
const failed = results.some((r) => r.ok === false);
const pending = results.some((r) => r.ok === null);
if (failed) finish(1, "failed", outDir);
if (pending) finish(2, "pending_verification", outDir);
finish(0, "passed", outDir);
