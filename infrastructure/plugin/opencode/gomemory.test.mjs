import { test } from 'node:test';
import assert from 'node:assert/strict';
import { GomemoryPlugin } from './gomemory.ts';

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
