import { test } from 'node:test';
import assert from 'node:assert/strict';
import plugin, { GomemoryPlugin, __testing } from './gomemory.ts';

async function fixture() {
  const calls = [];
  let messages = [];
  let recovery = '';
  const shell = (_strings, _bin, args) => {
    let input = '';
    const proc = {
      cwd: () => proc, quiet: () => proc,
      stdin: { getWriter: () => ({ write: async bytes => { input += new TextDecoder().decode(bytes); }, close: async () => {} }) },
      text: async () => { calls.push({ args, input }); return args.join(' ') === 'hook post-compact' ? recovery : ''; },
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
    { plan: 'Plan msg_003' }, { files: ['msg_003.go'], commands: [] },
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

// ctx falso con la forma de @opencode/plugin@2.0.16: suscripción de eventos
// como iterador asíncrono, ganchos por dominio y un exec que registra llamadas.
async function v2Fixture({ directory = '/proj', outputs = {}, owners } = {}) {
  const calls = [];
  const out = { context: 'CTX', 'hook compaction-context': 'COMPACT', ...outputs };
  const exec = async (bin, args, opts) => {
    calls.push({ args, input: opts?.input ?? '', cwd: opts?.cwd });
    if (out.fail?.includes(args.join(' '))) throw new Error('boom');
    return out[args.join(' ')] ?? '';
  };
  let messages = [];
  const queue = [];
  let wake = () => {};
  const sessionHooks = {};
  const toolHooks = {};
  const reg = { dispose: async () => {} };
  const ctx = {
    location: { directory },
    event: {
      subscribe: ({ signal }) => ({
        async *[Symbol.asyncIterator]() {
          while (!signal?.aborted) {
            if (queue.length) { yield queue.shift(); continue; }
            await new Promise(r => { wake = r; signal?.addEventListener('abort', r, { once: true }); });
          }
        },
      }),
    },
    session: {
      hook: async (name, fn) => { sessionHooks[name] = fn; return reg; },
      context: async () => (typeof messages === 'function' ? messages() : messages),
      // owners: sessionID → directorio dueño. Sin owners, el fake no expone
      // get (host sin la API): el plugin asume que la sesión es suya.
      ...(owners ? { get: async ({ sessionID }) => {
        if (!(sessionID in owners)) throw new Error('session not found');
        return { id: sessionID, location: { directory: owners[sessionID] } };
      } } : {}),
    },
    tool: { hook: async (name, fn) => { toolHooks[name] = fn; return reg; } },
  };
  const cleanup = await __testing.createV2Setup(exec)(ctx);
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
  return { calls, emit, system, sessionHooks, toolHooks, cleanup, setMessages: v => { messages = v; },
    hookCalls: name => calls.filter(c => c.args[0] === 'hook' && c.args[1] === name) };
}

const v2Assistant = (id, tools, agent = 'build') => ({ type: 'assistant', id, agent, content: tools.map(([name, input]) => (
  { type: 'tool', id: `t_${id}_${name}`, name, state: { status: 'completed', input, content: [{ type: 'text', text: 'ok' }] } })) });

test('v2: session.created starts a session only for this directory', async () => {
  const f = await v2Fixture();
  await f.emit('session.created', { sessionID: 's1' }, '/otro');
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'session start').length, 0);
  await f.emit('session.created');
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'session start').length, 1);
  assert.equal(f.calls[0].cwd, '/proj');
  await f.cleanup();
});

test('v2: idle checkpoints edited files and commands once', async () => {
  const f = await v2Fixture();
  f.setMessages([{ type: 'user', id: 'msg_001', text: 'hola' }, v2Assistant('msg_002', [['write', { filePath: 'a.go' }], ['bash', { command: 'ls' }]])]);
  await f.emit('session.idle');
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: ['a.go'], commands: ['ls'] }]);
  await f.emit('session.idle');
  assert.equal(f.hookCalls('turn-end').length, 1);
  await f.cleanup();
});

test('v2: context hook injects protocol and project memory as text parts', async () => {
  const f = await v2Fixture();
  const parts = await f.system('context');
  assert.ok(parts.some(t => t.includes('Memory Protocol')));
  assert.ok(parts.includes('## Project Memory (gomemory)\n\nCTX'));
  await f.cleanup();
});

test('v2: cleanup stops the subscription and closes the session', async () => {
  const f = await v2Fixture();
  await f.cleanup();
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'session end').length, 1);
  await f.emit('session.created');
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'session start').length, 0);
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
  await f.cleanup();
});

// OpenCode 2.0.16 real no emite session.idle al terminar un turno: emite
// session.execution.succeeded|failed|interrupted, sin location (verificado con
// el binario, feature 032). Sin esta traducción el checkpoint nunca corría.
for (const end of ['session.execution.succeeded', 'session.execution.failed', 'session.execution.interrupted']) test(`v2: ${end} checkpoints like idle`, async () => {
  const f = await v2Fixture();
  f.setMessages([v2Assistant('msg_010', [['edit', { filePath: 'b.go' }]])]);
  await f.emit(end, { sessionID: 's1' }, null);
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: ['b.go'], commands: [] }]);
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
  assert.deepEqual(f.hookCalls('turn-end').map(c => JSON.parse(c.input)), [{ files: ['hola.txt'], commands: ['ls'] }]);
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
  const before = f.calls.filter(c => c.args.join(' ') === 'session end');
  assert.deepEqual(f.calls.filter(c => !before.includes(c)), []);
  await f.cleanup();
});

test('v2: session.created uses the session location, not the envelope', async () => {
  const f = await v2Fixture({ owners: {} });
  await f.emit('session.created', { sessionID: 'sB', location: { directory: '/projB' } }, null);
  await f.emit('session.created', { sessionID: 'sP', location: { directory: '/proj' } }, '/home');
  assert.equal(f.calls.filter(c => c.args.join(' ') === 'session start').length, 1);
  assert.ok((await f.system('context', 'sP')).length > 0);
  assert.deepEqual(await f.system('context', 'sB'), []);
  await f.cleanup();
});
