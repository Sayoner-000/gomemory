import { test, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import plugin, { GomemoryPlugin, __testing } from './gomemory.ts';

async function fixture({ decide = () => '', expandOut = () => '' } = {}) {
  const calls = [];
  let messages = [];
  let recovery = '';
  const shell = (_strings, _bin, args) => {
    let input = '';
    const proc = {
      cwd: () => proc, quiet: () => proc,
      stdin: { getWriter: () => ({ write: async bytes => { input += new TextDecoder().decode(bytes); }, close: async () => {} }) },
      text: async () => {
        calls.push({ args, input });
        if (args.join(' ') === 'hook socratica-decide') return decide(JSON.parse(input));
        if (args[0] === 'hook' && args[1] === 'expand') return expandOut(args[2], JSON.parse(input || '{}'));
        return args.join(' ') === 'hook post-compact' ? recovery : '';
      },
    };
    return proc;
  };
  const hooks = await GomemoryPlugin({ $: shell, directory: '/tmp', client: { session: { messages: async () => ({ data: messages }) } } });
  return { hooks, calls, setMessages: value => { messages = value; }, setRecovery: value => { recovery = value; },
    event: (type, sessionID = 's1') => hooks.event({ event: { type, properties: { sessionID, info: { id: sessionID } } } }) };
}
const message = id => ({ info: { id, role: 'assistant', mode: 'plan' }, parts: [
  { type: 'text', text: `Plan ${id}` },
  { type: 'tool', tool: 'write', state: { status: 'completed', input: { filePath: `${id}.go` } } },
] });
const checkpoints = f => f.calls.filter(c => ['plan-approved', 'turn-end'].includes(c.args[1]));

test('compaction removes checkpoint marker without replaying surviving old messages', async () => {
  const f = await fixture();
  f.setMessages([message('msg_001'), message('msg_002')]);
  await f.event('session.idle');
  const before = checkpoints(f).length;
  f.setMessages([message('msg_001')]);
  await f.event('session.idle');
  assert.equal(checkpoints(f).length, before);
  f.setMessages([message('msg_001'), message('msg_003')]);
  await f.event('session.idle');
  assert.deepEqual(checkpoints(f).slice(before).map(c => JSON.parse(c.input)), [
    { plan: 'Plan msg_003' }, { files: ['msg_003.go'], commands: [], client: 'opencode', session_id: 's1' },
  ]);
  await f.event('session.idle');
  assert.equal(checkpoints(f).length, before + 2);
});

test('latest complete recovery is consumed once and isolated by session', async () => {
  const f = await fixture();
  f.setRecovery('old'); await f.event('session.compacted');
  f.setRecovery('new'); await f.event('session.compacted');
  const transform = async sessionID => { const out = { system: [] }; await f.hooks['experimental.chat.system.transform']({ sessionID }, out); return out.system; };
  assert.ok(!(await transform('s2')).includes('new'));
  const out = await transform('s1');
  assert.ok(out.includes('new')); assert.ok(!out.includes('old'));
  assert.ok(!(await transform('s1')).includes('new'));
});

for (const cleanup of ['session.deleted', 'dispose']) test(`${cleanup} releases session checkpoint and recovery`, async () => {
  const f = await fixture();
  f.setMessages([message('msg_001')]); await f.event('session.idle');
  f.setRecovery('pending'); await f.event('session.compacted');
  if (cleanup === 'dispose') await f.hooks.dispose(); else await f.event(cleanup);
  const out = { system: [] }; await f.hooks['experimental.chat.system.transform']({ sessionID: 's1' }, out);
  assert.ok(!out.system.includes('pending'));
  const before = checkpoints(f).length;
  await f.event('session.idle');
  assert.equal(checkpoints(f).length, before + 2);
});

test('task completion sends learning text, other tools do not', async () => {
  const f = await fixture();
  await f.hooks['tool.execute.after']({ tool: 'bash' }, { output: 'ignored' });
  await f.hooks['tool.execute.after']({ tool: 'task' }, { output: '## Learnings\n- A useful learning from the task' });
  const captures = f.calls.filter(c => c.args[1] === 'subagent-stop');
  assert.equal(captures.length, 1);
  assert.equal(JSON.parse(captures[0].input).last_assistant_message, '## Learnings\n- A useful learning from the task');
});

// ── Export dual y rama OpenCode 2.x (feature 032) ───────────────────────────

test('default export is the dual v1+v2 definition', () => {
  assert.equal(plugin.id, 'gomemory');
  assert.equal(typeof plugin.setup, 'function');
  assert.equal(plugin.server, GomemoryPlugin);
});

const settle = () => new Promise(r => setTimeout(r, 15));

// Comparación por prefijo semántico de argumentos (H-09, FR-024): producción
// añade argumentos como --conversation=<id>, así que comparar la llamada
// completa con 'session start' dejaba de detectar el inicio real.
const startsWith = (args, prefix) => prefix.split(' ').every((w, i) => args[i] === w);

// Fixtures v2 vivas: afterEach las libera aunque un assert falle a mitad de
// prueba, para que una suscripción abierta no contamine a las siguientes.
const liveV2 = new Set();
afterEach(async () => {
  for (const f of [...liveV2]) await f.cleanup();
});

// ctx falso con la forma de @opencode/plugin@2.0.16: suscripción de eventos
// como iterador asíncrono, ganchos por dominio y un exec que registra llamadas.
// Opciones del fixture (feature 037):
//   execImpl(args, opts)  sustituye la respuesta de mem (p. ej. IO lento que
//                         solo termina al abortarse la señal recibida)
//   failHook              nombre del registro cuyo hook() rechaza en setup
//   disposeFail           nombre del registro cuyo dispose() rechaza
//   subscribeFail         la suscripción a eventos rechaza al iterar
//   options               opciones internas de createV2Setup (closeBudgetMs)
async function v2Fixture({ directory = '/proj', outputs = {}, owners, execImpl, failHook, disposeFail, subscribeFail, options, allowSetupFailure } = {}) {
  const calls = [];
  const failures = [];
  const out = { context: 'CTX', 'hook compaction-context': 'COMPACT', ...outputs };
  const exec = async (bin, args, opts) => {
    calls.push({ args, input: opts?.input ?? '', cwd: opts?.cwd, signal: opts?.signal });
    const failing = out.fail?.find(prefix => startsWith(args, prefix));
    if (failing) { failures.push(failing); throw new Error('boom'); }
    if (execImpl) {
      const res = await execImpl(args, opts);
      if (res !== undefined) return res;
    }
    return out[args.join(' ')] ?? '';
  };
  let messages = [];
  const queue = [];
  let wake = () => {};
  const sessionHooks = {};
  const toolHooks = {};
  const disposed = [];
  let subscriptionSignal;
  const register = (name, fn, target) => {
    if (failHook === name) throw new Error(`registro ${name} rechazado`);
    target[name] = fn;
    return { dispose: async () => {
      disposed.push(name);
      if (disposeFail === name) throw new Error(`dispose ${name} falló`);
    } };
  };
  const ctx = {
    location: { directory },
    event: {
      subscribe: ({ signal }) => {
        subscriptionSignal = signal;
        return {
          async *[Symbol.asyncIterator]() {
            if (subscribeFail) throw new Error('suscripción rechazada');
            while (!signal?.aborted) {
              if (queue.length) { yield queue.shift(); continue; }
              await new Promise(r => { wake = r; signal?.addEventListener('abort', r, { once: true }); });
            }
          },
        };
      },
    },
    session: {
      hook: async (name, fn) => register(name, fn, sessionHooks),
      context: async () => (typeof messages === 'function' ? messages() : messages),
      // owners: sessionID → directorio dueño. Sin owners, el fake no expone
      // get (host sin la API): el plugin asume que la sesión es suya.
      ...(owners ? { get: async ({ sessionID }) => {
        if (!(sessionID in owners)) throw new Error('session not found');
        return { id: sessionID, location: { directory: owners[sessionID] } };
      } } : {}),
    },
    tool: { hook: async (name, fn) => register(name, fn, toolHooks) },
  };
  let rawCleanup;
  let setupError;
  try {
    rawCleanup = await __testing.createV2Setup(exec, options)(ctx);
  } catch (err) {
    if (!allowSetupFailure) throw err;
    setupError = err;
    rawCleanup = async () => {};
  }
  let cleaned;
  const cleanup = () => {
    liveV2.delete(fixture);
    return (cleaned ??= rawCleanup());
  };
  const emit = async (type, data = { sessionID: 's1' }, dir = directory) => {
    // dir === null: evento sin location, como session.execution.* en 2.0.16.
    queue.push({ type, data, ...(dir === null ? {} : { location: { directory: dir } }) });
    wake();
    await settle();
  };
  const system = async (name, sessionID = 's1') => {
    const ev = { sessionID, system: [], messages: [] };
    await sessionHooks[name](ev);
    return ev.system.map(p => { assert.equal(p.type, 'text'); return p.text; });
  };
  const fixture = { calls, failures, emit, system, sessionHooks, toolHooks, cleanup, rawCleanup, disposed, setupError,
    subscriptionSignal: () => subscriptionSignal, setMessages: v => { messages = v; },
    hookCalls: name => calls.filter(c => c.args[0] === 'hook' && c.args[1] === name),
    callsTo: prefix => calls.filter(c => startsWith(c.args, prefix)),
    // Inicio de sesión de memoria de una conversación concreta y en el
    // directorio del proyecto: comando, identidad y proyecto a la vez.
    sessionStarts: id => calls.filter(c => startsWith(c.args, 'session start')
      && c.args.includes(`--conversation=${id}`) && c.cwd === directory) };
  liveV2.add(fixture);
  return fixture;
}

const v2Assistant = (id, tools, agent = 'build') => ({ type: 'assistant', id, agent, content: tools.map(([name, input]) => (
  { type: 'tool', id: `t_${id}_${name}`, name, state: { status: 'completed', input, content: [{ type: 'text', text: 'ok' }] } })) });

test('v2: session.created starts a session only for this directory', async () => {
  const f = await v2Fixture();
  await f.emit('session.created', { sessionID: 's1' }, '/otro');
  assert.equal(f.callsTo('session start').length, 0);
  await f.emit('session.created');
  assert.equal(f.callsTo('session start').length, 1);
  assert.equal(f.sessionStarts('s1').length, 1);
  await f.cleanup();
});

test('v2: idle checkpoints edited files and commands once', async () => {
  const f = await v2Fixture();
  f.setMessages([{ type: 'user', id: 'msg_001', text: 'hola' }, v2Assistant('msg_002', [['write', { filePath: 'a.go' }], ['bash', { command: 'ls' }]])]);
  await f.emit('session.idle');
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: ['a.go'], commands: ['ls'], client: 'opencode', session_id: 's1' }]);
  await f.emit('session.idle');
  assert.equal(f.hookCalls('turn-end').length, 1);
  await f.cleanup();
});

// FR-041b: el heartbeat HCD corre en cada turno, también sin actividad; mem
// descarta el checkpoint vacío después de registrarlo.
test('v2: idle without activity still reaches turn-end for the HCD heartbeat', async () => {
  const f = await v2Fixture();
  f.setMessages([{ type: 'user', id: 'msg_001', text: 'hola' }, v2Assistant('msg_002', [])]);
  await f.emit('session.idle');
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: [], commands: [], client: 'opencode', session_id: 's1' }]);
  await f.cleanup();
});

test('v2: context hook injects protocol and project memory as text parts', async () => {
  const f = await v2Fixture();
  const parts = await f.system('context');
  assert.ok(parts.some(t => t.includes('Memory Protocol')));
  assert.ok(parts.includes('## Project Memory (gomemory)\n\nCTX'));
  await f.cleanup();
});

test('v2: context asks for the octopus policy as opencode-v2, v1 keeps opencode', async () => {
  const f = await v2Fixture();
  await f.system('context');
  assert.deepEqual(f.hookCalls('octopus-delegation-policy').map(c => c.args[2]), ['opencode-v2']);
  await f.cleanup();
  const v1 = await fixture();
  await v1.hooks['experimental.chat.system.transform']({ sessionID: 's1' }, { system: [] });
  assert.deepEqual(v1.calls.filter(c => c.args[1] === 'octopus-delegation-policy').map(c => c.args[2]), ['opencode']);
});

test('v2: the protocol names MCP tools as Code Mode sees them, v1 keeps prefixed names', async () => {
  const f = await v2Fixture();
  const protocol = (await f.system('context')).find(t => t.includes('Memory Protocol'));
  assert.ok(protocol.includes('tools.gomemory.save_memory'));
  assert.ok(protocol.includes('tools["codebase-memory-mcp"].search_graph'));
  assert.ok(protocol.includes('`execute`'));
  assert.ok(protocol.includes('codemode'));
  assert.doesNotMatch(protocol.replace(/`gomemory_<tool>`/g, ''), /\bgomemory_[a-z]/);
  await f.cleanup();
  const v1 = await fixture();
  const out = { system: [] };
  await v1.hooks['experimental.chat.system.transform']({ sessionID: 's1' }, out);
  const v1Protocol = out.system.find(t => t.includes('Memory Protocol'));
  assert.ok(v1Protocol.includes('gomemory_save_memory'));
  assert.ok(!v1Protocol.includes('tools.gomemory.'));
});

test('v2: cleanup stops the subscription and closes the session', async () => {
  const f = await v2Fixture();
  await f.cleanup();
  assert.equal(f.callsTo('session end').length, 1);
  await f.emit('session.created');
  assert.equal(f.callsTo('session start').length, 0);
});

test('v2: prompt hook records provenance', async () => {
  const f = await v2Fixture();
  await f.sessionHooks.prompt({ sessionID: 's1', prompt: { text: 'hola' } });
  await f.sessionHooks.prompt({ sessionID: 's1', prompt: { text: '  ' } });
  assert.deepEqual(f.hookCalls('prompt').map(c => JSON.parse(c.input)), [{ prompt: 'hola' }]);
  await f.cleanup();
});

test('v2: compaction hook preserves session memory in the summary request', async () => {
  const f = await v2Fixture();
  assert.deepEqual(await f.system('compaction'), ['COMPACT']);
  assert.equal(f.hookCalls('post-compact').length, 0);
  await f.cleanup();
});

test('v2: compaction.ended stores summary and delivers recovery once per session', async () => {
  const f = await v2Fixture({ outputs: { 'hook post-compact': 'RECOVERY' } });
  f.setMessages([{ type: 'compaction', id: 'msg_009', status: 'completed', reason: 'auto', summary: 'Resumen X', recent: '' }]);
  await f.emit('session.compaction.ended');
  assert.deepEqual(f.hookCalls('compact-summary').map(c => JSON.parse(c.input)), [{ summary: 'Resumen X' }]);
  assert.ok(!(await f.system('context', 's2')).includes('RECOVERY'));
  assert.ok((await f.system('context', 's1')).includes('RECOVERY'));
  assert.ok(!(await f.system('context', 's1')).includes('RECOVERY'));
  await f.cleanup();
});

test('v2: completed task result reaches subagent-stop, other tools and errors do not', async () => {
  const f = await v2Fixture();
  const after = f.toolHooks['execute.after'];
  await after({ tool: 'bash', sessionID: 's1', status: 'completed', result: { content: 'ignored' } });
  await after({ tool: 'task', sessionID: 's1', status: 'error', error: {} });
  await after({ tool: 'task', sessionID: 's1', status: 'completed', result: { content: [{ type: 'text', text: '## Learnings' }, { type: 'text', text: '- X' }] } });
  assert.deepEqual(f.hookCalls('subagent-stop').map(c => JSON.parse(c.input)), [{ last_assistant_message: '## Learnings\n- X' }]);
  await f.cleanup();
});

test('v2: unreadable session messages leave a channel-error trace instead of a silent gap', async () => {
  const f = await v2Fixture();
  f.setMessages({ unexpected: true });
  await f.emit('session.idle');
  assert.equal(f.hookCalls('turn-end').length, 0);
  assert.equal(f.hookCalls('channel-error').length, 1);
  await f.cleanup();
});

test('v2: a failing mem never throws into OpenCode', async () => {
  const f = await v2Fixture({ outputs: { fail: ['context', 'hook prompt', 'session start'] } });
  await f.emit('session.created');
  await f.sessionHooks.prompt({ sessionID: 's1', prompt: { text: 'hola' } });
  await f.system('context');
  // El fallo inyectado tiene que haberse producido de verdad: sin esto, un
  // fixture que deja de coincidir con los argumentos pasa sin probar nada.
  for (const prefix of ['context', 'hook prompt', 'session start']) assert.ok(f.failures.includes(prefix), `sin fallo en ${prefix}`);
  await f.cleanup();
});

// OpenCode 2.0.16 real no emite session.idle al terminar un turno: emite
// session.execution.succeeded|failed|interrupted, sin location (verificado con
// el binario, feature 032). Sin esta traducción el checkpoint nunca corría.
for (const end of ['session.execution.succeeded', 'session.execution.failed', 'session.execution.interrupted']) test(`v2: ${end} checkpoints like idle`, async () => {
  const f = await v2Fixture();
  f.setMessages([v2Assistant('msg_010', [['edit', { filePath: 'b.go' }]])]);
  await f.emit(end, { sessionID: 's1' }, null);
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: ['b.go'], commands: [], client: 'opencode', session_id: 's1' }]);
  await f.emit('session.idle');
  assert.equal(f.hookCalls('turn-end').length, 1);
  await f.cleanup();
});

// Nombres reales de las tools de OpenCode 2.0.16 (leídos del gancho context
// del binario): shell reemplaza a bash y subagent reemplaza a task.
test('v2: shell commands reach the checkpoint like bash', async () => {
  const f = await v2Fixture();
  f.setMessages([v2Assistant('msg_020', [['write', { path: 'hola.txt', content: 'hola' }], ['shell', { command: 'ls', workdir: '/proj' }]])]);
  await f.emit('session.execution.succeeded', { sessionID: 's1' }, null);
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: ['hola.txt'], commands: ['ls'], client: 'opencode', session_id: 's1' }]);
  await f.cleanup();
});

test('v2: subagent result is captured like task', async () => {
  const f = await v2Fixture();
  await f.toolHooks['execute.after']({ tool: 'subagent', sessionID: 's1', status: 'completed', result: { content: '## Learnings\n- Y' } });
  assert.deepEqual(f.hookCalls('subagent-stop').map(c => JSON.parse(c.input)), [{ last_assistant_message: '## Learnings\n- Y' }]);
  await f.cleanup();
});

// Un servicio de OpenCode 2.x aloja varias ubicaciones y entrega los eventos y
// los ganchos de sesión a TODAS las instancias del plugin (reproducido con el
// binario 2.0.16 y dos proyectos: la instancia de B registraba el turno de A
// con el cwd de B). Cada instancia atiende solo las sesiones de su directorio.
test('v2: foreign session events without location are ignored', async () => {
  const f = await v2Fixture({ owners: { sA: '/projA', sP: '/proj' } });
  f.setMessages([v2Assistant('msg_030', [['write', { path: 'a.txt' }]])]);
  await f.emit('session.execution.succeeded', { sessionID: 'sA' }, null);
  assert.equal(f.hookCalls('turn-end').length, 0);
  await f.emit('session.execution.succeeded', { sessionID: 'sP' }, null);
  assert.equal(f.hookCalls('turn-end').length, 1);
  await f.cleanup();
});

test('v2: foreign sessions get no context, prompt, compaction or subagent calls', async () => {
  const f = await v2Fixture({ owners: { sA: '/projA' } });
  assert.deepEqual(await f.system('context', 'sA'), []);
  assert.deepEqual(await f.system('compaction', 'sA'), []);
  await f.sessionHooks.prompt({ sessionID: 'sA', prompt: { text: 'hola' } });
  await f.toolHooks['execute.after']({ tool: 'subagent', sessionID: 'sA', status: 'completed', result: { content: 'x' } });
  const before = f.callsTo('session end');
  assert.deepEqual(f.calls.filter(c => !before.includes(c)), []);
  await f.cleanup();
});

test('v2: session.created uses the session location, not the envelope', async () => {
  const f = await v2Fixture({ owners: {} });
  await f.emit('session.created', { sessionID: 'sB', location: { directory: '/projB' } }, null);
  await f.emit('session.created', { sessionID: 'sP', location: { directory: '/proj' } }, '/home');
  assert.equal(f.callsTo('session start').length, 1);
  assert.equal(f.sessionStarts('sP').length, 1);
  assert.ok((await f.system('context', 'sP')).length > 0);
  assert.deepEqual(await f.system('context', 'sB'), []);
  await f.cleanup();
});

// ── Feature 037 · H-05: el consumidor v2 recibe la salida reducida ─────────
// La fábrica v1 escribía la reducción en un objeto temporal que nadie leía:
// event.result seguía con el texto original. Estas pruebas observan
// event.result DESPUÉS del gancho, que es lo que recibe OpenCode.

const BIG = 'x'.repeat(9000);
const compressing = { 'hook tool-output --enabled': 'true', 'hook tool-output opencode': JSON.stringify({ output: 'REDUCIDO' }) };

const afterTool = async (f, ev) => {
  const event = { tool: 'shell', sessionID: 's1', status: 'completed', id: 'c1', ...ev };
  await f.toolHooks['execute.after'](event);
  return event;
};

test('v2 (037): content string reducido llega al consumidor y conserva los demás campos', async () => {
  const f = await v2Fixture({ outputs: compressing });
  const ev = await afterTool(f, { result: { content: BIG, title: 'ls', metadata: { exit: 0 } } });
  assert.deepEqual(ev.result, { content: 'REDUCIDO', title: 'ls', metadata: { exit: 0 } });
  await f.cleanup();
});

test('v2 (037): partes mixtas: primera textual reducida con sus metadatos, no textuales en orden', async () => {
  const f = await v2Fixture({ outputs: compressing });
  const content = [
    { type: 'text', text: 'a'.repeat(5000), id: 'p1', synthetic: true },
    { type: 'image', url: 'file:///img.png', id: 'p2' },
    { type: 'text', text: 'b'.repeat(5000), id: 'p3' },
    { type: 'file', path: 'x.go', id: 'p4' },
  ];
  const ev = await afterTool(f, { result: { content } });
  assert.deepEqual(ev.result.content, [
    { type: 'text', text: 'REDUCIDO', id: 'p1', synthetic: true },
    { type: 'image', url: 'file:///img.png', id: 'p2' },
    { type: 'file', path: 'x.go', id: 'p4' },
  ]);
  await f.cleanup();
});

test('v2 (037): output string reducido; output objeto nunca se reemplaza por texto', async () => {
  const f = await v2Fixture({ outputs: compressing });
  const ev = await afterTool(f, { result: { output: BIG, title: 't' } });
  assert.deepEqual(ev.result, { output: 'REDUCIDO', title: 't' });
  const objectOutput = { output: { rows: [BIG] } };
  const ev2 = await afterTool(f, { result: objectOutput });
  assert.equal(ev2.result, objectOutput);
  assert.deepEqual(ev2.result.output, { rows: [BIG] });
  await f.cleanup();
});

test('v2 (037): sin compresión, sin ganancia, incompleto, ajeno o sin dueño: resultado intacto', async () => {
  const cases = [
    { name: 'desactivada', outputs: { 'hook tool-output --enabled': 'false' } },
    { name: 'sin ganancia', outputs: { 'hook tool-output --enabled': 'true', 'hook tool-output opencode': JSON.stringify({ output: BIG }) } },
    { name: 'protegida (el hook devuelve vacío)', outputs: { 'hook tool-output --enabled': 'true', 'hook tool-output opencode': '' } },
    { name: 'no completado', outputs: compressing, ev: { status: 'error' } },
    { name: 'sesión ajena', outputs: compressing, owners: { s1: '/otro' } },
    { name: 'dueño no resuelto', outputs: compressing, owners: {} },
  ];
  for (const c of cases) {
    const f = await v2Fixture({ outputs: c.outputs, owners: c.owners });
    const result = { content: BIG };
    const ev = await afterTool(f, { result, ...(c.ev ?? {}) });
    assert.equal(ev.result, result, c.name);
    assert.equal(ev.result.content, BIG, c.name);
    await f.cleanup();
  }
});

test('v2 (037): una reducción que termina después del cierre no se publica', async () => {
  let release;
  const gate = new Promise(r => { release = r; });
  const f = await v2Fixture({ outputs: compressing, execImpl: async (args) => {
    if (startsWith(args, 'hook tool-output opencode')) { await gate; }
  } });
  const result = { content: BIG };
  const event = { tool: 'shell', sessionID: 's1', status: 'completed', id: 'c1', result };
  const pending = f.toolHooks['execute.after'](event);
  await settle();
  const closing = f.cleanup();
  release();
  await pending;
  await closing;
  assert.equal(event.result, result);
});

// ── Feature 037 · H-06: ciclo de vida cancelable y rollback de setup ───────

for (const failHook of ['prompt', 'context', 'compaction', 'execute.before', 'execute.after']) test(`v2 (037): fallo al registrar ${failHook} revierte lo propio sin cerrar la sesión ajena`, async () => {
  const f = await v2Fixture({ failHook, allowSetupFailure: true });
  assert.match(String(f.setupError?.message), new RegExp(`registro ${failHook} rechazado`));
  assert.equal(f.subscriptionSignal()?.aborted, true, 'la suscripción propia debe abortarse');
  const order = ['prompt', 'context', 'compaction', 'execute.before', 'execute.after'];
  assert.deepEqual(f.disposed, order.slice(0, order.indexOf(failHook)), 'se liberan los registros previos');
  assert.equal(f.callsTo('session end').length, 0, 'el rollback no cierra la sesión de memoria');
});

test('v2 (037): cierre con evento en curso cancela el IO, espera y no deja operaciones', async () => {
  let started;
  const ioStarted = new Promise(r => { started = r; });
  let abortedSeen = false;
  const f = await v2Fixture({ execImpl: (args, opts) => {
    if (!startsWith(args, 'hook turn-end')) return undefined;
    started();
    return new Promise((_, reject) => {
      opts.signal?.addEventListener('abort', () => { abortedSeen = true; reject(new Error('abortado')); }, { once: true });
    });
  } });
  f.setMessages([v2Assistant('msg_100', [['write', { filePath: 'a.go' }]])]);
  f.emit('session.idle');
  await ioStarted;
  const t0 = Date.now();
  const res = await f.cleanup();
  assert.ok(Date.now() - t0 <= 5000, 'cierre en <= 5 s');
  assert.equal(abortedSeen, true, 'el subproceso en curso recibió la cancelación');
  assert.equal(res?.ok, true, JSON.stringify(res));
  assert.equal(f.callsTo('session end').length, 1);
  assert.equal(f.callsTo('session end')[0].signal?.aborted, false, 'el cierre de sesión no usa la señal ya abortada');
  const before = f.calls.length;
  await f.sessionHooks.prompt({ sessionID: 's1', prompt: { text: 'tarde' } });
  await f.toolHooks['execute.after']({ tool: 'task', sessionID: 's1', status: 'completed', result: { content: 'x' } });
  await f.emit('session.created');
  assert.equal(f.calls.length, before, 'después de closing no hay IO nuevo');
});

test('v2 (037): cleanup repetido comparte el resultado y no duplica el cierre', async () => {
  const f = await v2Fixture();
  const [a, b] = await Promise.all([f.rawCleanup(), f.rawCleanup()]);
  assert.deepEqual(a, b);
  assert.equal(f.callsTo('session end').length, 1);
  assert.equal(f.disposed.length, 5);
  liveV2.delete(f);
});

test('v2 (037): un dispose que falla no impide liberar el resto y se informa depurado', async () => {
  const f = await v2Fixture({ disposeFail: 'context' });
  const res = await f.cleanup();
  assert.deepEqual(f.disposed, ['prompt', 'context', 'compaction', 'execute.before', 'execute.after']);
  assert.equal(res?.ok, false);
  assert.equal(res?.code, 'OPENCODE_CLEANUP_FAILED');
  assert.ok(!JSON.stringify(res).includes('dispose context falló'), 'sin el mensaje crudo');
  assert.equal(f.callsTo('session end').length, 1);
});

test('v2 (037): una suscripción rechazada no rompe el cierre', async () => {
  const f = await v2Fixture({ subscribeFail: true });
  const res = await f.cleanup();
  assert.equal(res?.ok, true, JSON.stringify(res));
  assert.equal(f.disposed.length, 5);
});

test('v2 (037): IO que ignora la cancelación agota el plazo y el cierre no se declara exitoso', async () => {
  let started;
  const ioStarted = new Promise(r => { started = r; });
  const f = await v2Fixture({ options: { closeBudgetMs: 200 }, execImpl: (args) => {
    if (!startsWith(args, 'hook turn-end')) return undefined;
    started();
    return new Promise(() => {});
  } });
  f.setMessages([v2Assistant('msg_200', [['write', { filePath: 'b.go' }]])]);
  f.emit('session.idle');
  await ioStarted;
  const t0 = Date.now();
  const res = await f.cleanup();
  assert.ok(Date.now() - t0 < 2000, 'respeta el plazo configurado');
  assert.equal(res?.ok, false);
  assert.equal(res?.code, 'OPENCODE_CLEANUP_FAILED');
});

// execFileExec real (037, H-06): la cancelación termina el subproceso y la
// promesa solo se rechaza cuando el hijo ya salió.
test('v2 (037): execFileExec cancela un subproceso real y respeta la gracia antes de SIGKILL', { skip: process.platform === 'win32' }, async () => {
  const { execFileExec } = __testing;
  const pre = new AbortController();
  pre.abort();
  await assert.rejects(execFileExec('/bin/sh', ['-c', 'echo no'], { cwd: '/', signal: pre.signal }), { name: 'AbortError' });

  const polite = new AbortController();
  const t0 = Date.now();
  const p1 = execFileExec('/bin/sh', ['-c', 'sleep 30'], { cwd: '/', signal: polite.signal });
  setTimeout(() => polite.abort(), 50);
  await assert.rejects(p1, { name: 'AbortError' });
  assert.ok(Date.now() - t0 < 1000, 'SIGTERM basta para un proceso cortés');

  const stubborn = new AbortController();
  const t1 = Date.now();
  const p2 = execFileExec('/bin/sh', ['-c', 'trap "" TERM; sleep 30 & wait'], { cwd: '/', signal: stubborn.signal });
  setTimeout(() => stubborn.abort(), 50);
  await assert.rejects(p2, { name: 'AbortError' });
  const took = Date.now() - t1;
  assert.ok(took >= 1000 && took < 3000, `SIGKILL tras la gracia de 1 s (tardó ${took} ms)`);
});

// ── Socratica (feature 039, US6) ────────────────────────────────────────────
// El plugin no decide nada: aplica la respuesta de `mem hook socratica-decide`.

const textPart = text => ({ id: 'prt_1', sessionID: 's1', messageID: 'msg_1', type: 'text', text });
const decision = d => () => JSON.stringify({ capture: false, forward: '', forwarded: false, directive: '', ...d });

test('socratica: capture prepends the directive part and records the original prompt', async () => {
  const f = await fixture({ decide: decision({ capture: true, directive: 'DIRECTIVA' }) });
  const output = { parts: [textPart('envía el informe trimestral al cliente')] };
  await f.hooks['chat.message']({ sessionID: 's1' }, output);
  assert.equal(output.parts.length, 2);
  assert.equal(output.parts[0].text, 'DIRECTIVA');
  assert.equal(output.parts[0].synthetic, true);
  assert.equal(output.parts[1].text, 'envía el informe trimestral al cliente');
  const decide = f.calls.find(c => c.args.join(' ') === 'hook socratica-decide');
  assert.deepEqual(JSON.parse(decide.input), { prompt: 'envía el informe trimestral al cliente', client: 'opencode', attachments: false });
  assert.ok(f.calls.some(c => c.args.join(' ') === 'hook prompt'));
});

test('socratica: forward rewrites the text with the decided prompt', async () => {
  const f = await fixture({ decide: decision({ forwarded: true, forward: '>hola' }) });
  const output = { parts: [textPart('>>hola')] };
  await f.hooks['chat.message']({ sessionID: 's1' }, output);
  assert.deepEqual(output.parts.map(p => p.text), ['>hola']);
});

test('socratica: a pass-through decision leaves the message intact', async () => {
  for (const text of ['/plan algo', 'texto aprobado']) {
    const f = await fixture({ decide: decision({}) });
    const output = { parts: [textPart(text)] };
    await f.hooks['chat.message']({ sessionID: 's1' }, output);
    assert.deepEqual(output.parts.map(p => p.text), [text]);
  }
});

test('socratica: attachments are reported to the decision', async () => {
  let seen;
  const f = await fixture({ decide: input => { seen = input; return decision({})(); } });
  await f.hooks['chat.message']({ sessionID: 's1' }, { parts: [textPart('mira esta imagen y dime qué ves'), { type: 'file', mime: 'image/png' }] });
  assert.equal(seen.attachments, true);
});

test('socratica: an unavailable or invalid decision changes nothing', async () => {
  for (const decide of [() => '', () => 'no es json']) {
    const f = await fixture({ decide });
    const output = { parts: [textPart('envía el informe trimestral al cliente')] };
    await f.hooks['chat.message']({ sessionID: 's1' }, output);
    assert.deepEqual(output.parts.map(p => p.text), ['envía el informe trimestral al cliente']);
  }
});

test('socratica: the plugin holds no capture rules of its own', async () => {
  const { readFile } = await import('node:fs/promises');
  const source = await readFile(new URL('./gomemory.ts', import.meta.url), 'utf8');
  for (const rule of ['LowInfoMaxWords', 'ContinuationWindow', 'GOMEMORY_SOCRATICA_DISPATCH', 'startsWith(">")', "startsWith('>')"]) {
    assert.ok(!source.includes(rule), `el plugin replica la regla ${rule}`);
  }
});

test('v2 socratica: the decision reaches the real prompt text', async () => {
  const cases = [
    [{ capture: true, directive: 'DIRECTIVA' }, 'envía el informe trimestral al cliente', 'DIRECTIVA\n\nenvía el informe trimestral al cliente'],
    [{ forwarded: true, forward: 'hola' }, '>hola', 'hola'],
    [{}, 'texto sin cambios', 'texto sin cambios'],
  ];
  for (const [d, text, want] of cases) {
    const decisionJSON = JSON.stringify({ capture: false, forward: '', forwarded: false, directive: '', ...d });
    const f = await v2Fixture({ outputs: { 'hook socratica-decide': decisionJSON } });
    const ev = { sessionID: 's1', prompt: { text } };
    await f.sessionHooks.prompt(ev);
    assert.equal(ev.prompt.text, want);
    assert.equal(f.hookCalls('prompt').length, 1);
    await f.cleanup();
  }
});

// ── mem expand (feature 040) ────────────────────────────────────────────────

const expandCalls = f => f.calls.filter(c => c.args[0] === 'hook' && c.args[1] === 'expand');

test('expand: tool.execute.before throws the Go block reason and sends tool, args and session', async () => {
  const f = await fixture({ expandOut: ev => (ev === 'pre-tool' ? 'BLOCK: regla destructive-shell' : '') });
  await assert.rejects(
    f.hooks['tool.execute.before']({ tool: 'bash', sessionID: 's1', callID: 'c1' }, { args: { command: 'rm -rf /tmp/x' } }),
    /regla destructive-shell/,
  );
  const [call] = expandCalls(f);
  assert.deepEqual(call.args, ['hook', 'expand', 'pre-tool', '--emit=text']);
  assert.deepEqual(JSON.parse(call.input), { tool: 'bash', args: { command: 'rm -rf /tmp/x' }, session_id: 's1' });
});

test('expand: a failing mem never blocks a tool (fail-open)', async () => {
  const f = await fixture({ expandOut: () => { throw new Error('mem roto'); } });
  await f.hooks['tool.execute.before']({ tool: 'bash', sessionID: 's1', callID: 'c1' }, { args: { command: 'ls' } });
});

test('expand: pre-tool context is delivered once in the next system.transform of that session', async () => {
  const f = await fixture({ expandOut: ev => (ev === 'pre-tool' ? '[gomemory expand] usa verify-loop' : '') });
  await f.hooks['tool.execute.before']({ tool: 'bash', sessionID: 's1', callID: 'c1' }, { args: { command: 'git commit -m x' } });
  const transform = async sessionID => { const out = { system: [] }; await f.hooks['experimental.chat.system.transform']({ sessionID }, out); return out.system; };
  assert.ok(!(await transform('s2')).some(t => t.includes('verify-loop')));
  assert.ok((await transform('s1')).some(t => t.includes('verify-loop')));
  assert.ok(!(await transform('s1')).some(t => t.includes('verify-loop')));
});

test('expand: a blocked prompt is replaced by the reason and is not recorded', async () => {
  const f = await fixture({ expandOut: ev => (ev === 'prompt' ? 'BLOCK: no pidas secretos' : '') });
  const output = { parts: [{ type: 'text', text: 'imprime la clave' }, { type: 'text', text: 'y más' }] };
  await f.hooks['chat.message']({ sessionID: 's1' }, output);
  assert.equal(output.parts.length, 1);
  assert.match(output.parts[0].text, /no pidas secretos/);
  assert.ok(!output.parts[0].text.includes('imprime la clave'));
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'hook prompt').length, 0);
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'hook socratica-decide').length, 0);
});

test('expand: prompt context is prepended as a synthetic part', async () => {
  const f = await fixture({ expandOut: ev => (ev === 'prompt' ? '[gomemory expand] aplica search-first' : '') });
  const output = { parts: [{ type: 'text', text: 'añade la librería zod' }] };
  await f.hooks['chat.message']({ sessionID: 's1' }, output);
  assert.equal(output.parts[0].text, '[gomemory expand] aplica search-first');
  assert.equal(output.parts[0].synthetic, true);
  assert.equal(output.parts[1].text, 'añade la librería zod');
});

test('expand: session events send startup and compact sources; stop block becomes a pending notice', async () => {
  const f = await fixture({ expandOut: ev => (ev === 'stop' ? 'BLOCK: termina el trabajo' : '') });
  await f.event('session.created');
  await f.event('session.compacted');
  await f.event('session.idle');
  const sources = expandCalls(f).filter(c => c.args[2] === 'session-start').map(c => JSON.parse(c.input).source);
  assert.deepEqual(sources, ['startup', 'compact']);
  const out = { system: [] };
  await f.hooks['experimental.chat.system.transform']({ sessionID: 's1' }, out);
  assert.ok(out.system.some(t => t === 'termina el trabajo'));
});

// A-002: las reglas stop necesitan el texto de la última respuesta; sin él
// ninguna regla puede coincidir.
test('expand: stop sends the last assistant text', async () => {
  const f = await fixture();
  f.setMessages([
    { info: { id: 'm1', role: 'assistant' }, parts: [{ type: 'text', text: 'viejo' }] },
    { info: { id: 'm2', role: 'user' }, parts: [{ type: 'text', text: 'sigue' }] },
    { info: { id: 'm3', role: 'assistant' }, parts: [{ type: 'text', text: 'Listo.' }, { type: 'tool', tool: 'bash', state: {} }, { type: 'text', text: 'DONE' }] },
  ]);
  await f.event('session.idle');
  const [stop] = expandCalls(f).filter(c => c.args[2] === 'stop');
  assert.deepEqual(JSON.parse(stop.input), { session_id: 's1', last_assistant_message: 'Listo.\nDONE' });
});

test('expand: stop without messages sends an empty last message', async () => {
  const f = await fixture();
  await f.event('session.idle');
  const [stop] = expandCalls(f).filter(c => c.args[2] === 'stop');
  assert.equal(JSON.parse(stop.input).last_assistant_message, '');
});

test('expand: post-tool observes bash only', async () => {
  const f = await fixture();
  await f.hooks['tool.execute.after']({ tool: 'bash', sessionID: 's1', args: { command: 'go test ./...' } }, { output: 'ok' });
  await f.hooks['tool.execute.after']({ tool: 'read', sessionID: 's1' }, { output: 'ok' });
  await settle();
  const posts = expandCalls(f).filter(c => c.args[2] === 'post-tool');
  assert.equal(posts.length, 1);
  assert.equal(JSON.parse(posts[0].input).args.command, 'go test ./...');
});

test('v2 expand: execute.before propagates the block to OpenCode and passes the input', async () => {
  const f = await v2Fixture({ outputs: { 'hook expand pre-tool --emit=text': 'BLOCK: regla destructive-shell' } });
  await assert.rejects(f.toolHooks['execute.before']({ tool: 'shell', sessionID: 's1', id: 't1', input: { command: 'rm -rf /' } }), /destructive-shell/);
  const [call] = f.callsTo('hook expand pre-tool');
  assert.deepEqual(JSON.parse(call.input), { tool: 'bash', args: { command: 'rm -rf /' }, session_id: 's1' });
});

test('v2 expand: execute.before allows when mem fails or the session is foreign', async () => {
  const failing = await v2Fixture({ outputs: { fail: ['hook expand'] } });
  await failing.toolHooks['execute.before']({ tool: 'shell', sessionID: 's1', id: 't1', input: { command: 'ls' } });
  const foreign = await v2Fixture({ owners: { sB: '/otro' }, outputs: { 'hook expand pre-tool --emit=text': 'BLOCK: x' } });
  await foreign.toolHooks['execute.before']({ tool: 'shell', sessionID: 'sB', id: 't1', input: { command: 'rm -rf /' } });
  assert.equal(foreign.callsTo('hook expand').length, 0);
});
