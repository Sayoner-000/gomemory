import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { assetName, verifyChecksum, parseOptions, EventStream, terminalMode } from '../dist/core.js';

test('assets coinciden con GoReleaser y se rechazan plataformas no publicadas', () => {
  assert.equal(assetName('darwin', 'arm64'), 'mem_darwin_arm64.tar.gz');
  assert.equal(assetName('win32', 'x64'), 'mem_windows_amd64.zip');
  assert.throws(() => assetName('win32', 'arm64'), /soportada/);
  assert.throws(() => assetName('linux', 'riscv64'), /soportada/);
});

test('checksum debe corresponder al asset completo antes de extraer', () => {
  const data = Buffer.from('binario');
  const digest = createHash('sha256').update(data).digest('hex');
  assert.doesNotThrow(() => verifyChecksum(data, `${digest}  mem_linux_amd64.tar.gz\n`, 'mem_linux_amd64.tar.gz'));
  assert.throws(() => verifyChecksum(Buffer.from('alterado'), `${digest}  mem_linux_amd64.tar.gz`, 'mem_linux_amd64.tar.gz'), /checksum/);
  assert.throws(() => verifyChecksum(data, `${digest}  otro.tar.gz`, 'mem_linux_amd64.tar.gz'), /checksum/);
});

test('opciones conservan rutas con espacios y validan antes de escribir', () => {
  const o = parseOptions(['mi proyecto', '--yes', '--agents', 'opencode,claude', '--no-motion']);
  assert.equal(o.target, 'mi proyecto');
  assert.deepEqual(o.agents, ['opencode', 'claude']);
  assert.equal(o.noMotion, true);
  assert.throws(() => parseOptions(['--scope', 'system']), /scope/);
  assert.throws(() => parseOptions(['--agents', 'inventado']), /agente/);
  assert.throws(() => parseOptions(['--version', '../../ruta']), /versión/);
});

test('NDJSON tolera chunks y Unicode, mantiene avisos y exige cierre', () => {
  const events = [];
  const stream = new EventStream(e => events.push(e));
  const data = Buffer.from([
    { contract_version: 1, type: 'start', name: 'Instalación' },
    { contract_version: 1, type: 'step', name: 'Integración', status: 'warn', manual: 'mem install --yes' },
    { contract_version: 1, type: 'complete', status: 'warn', exit_code: 0 }
  ].map(e => JSON.stringify(e)).join('\n')+'\n');
  for (const byte of data) stream.push(Buffer.from([byte]));
  assert.equal(stream.finish(0).status, 'warn');
  assert.equal(events[1].name, 'Integración');
  assert.throws(() => new EventStream(() => {}).finish(0), /cierre/);
  assert.throws(() => new EventStream(() => {}).push(Buffer.from('{"contract_version":2,"type":"start"}\n')), /contrato/);
});

test('un proceso fallido y un aviso nunca se anuncian como éxito', () => {
  const stream = new EventStream(() => {});
  stream.push(Buffer.from('{"contract_version":1,"type":"complete","status":"ok","exit_code":0}\n'));
  assert.throws(() => stream.finish(1), /salida/);
  assert.throws(() => stream.push(Buffer.from('{"contract_version":1,"type":"step","status":"ok","name":"tarde"}\n')), /cierre/);
});

test('el resultado final no puede ocultar avisos previos', () => {
  const stream = new EventStream(() => {});
  stream.push(Buffer.from('{"contract_version":1,"type":"step","status":"warn","name":"MCP"}\n'));
  stream.push(Buffer.from('{"contract_version":1,"type":"complete","status":"ok","exit_code":0}\n'));
  assert.throws(() => stream.finish(0), /avisos/);
});

test('un cierre fail no puede devolver éxito aunque el proceso termine en cero', () => {
  const stream = new EventStream(() => {});
  stream.push(Buffer.from('{"contract_version":1,"type":"complete","status":"fail","exit_code":0}\n'));
  assert.throws(() => stream.finish(0), /fallida/);
});

test('sin agentes se expresa explícitamente sin activar autodetección', () => {
  const options = parseOptions(['--agents', 'none', '--yes']);
  assert.deepEqual(options.agents, []);
  assert.equal(options.agentsExplicit, true);
});

test('CI y pipes no preguntan, NO_COLOR y movimiento son independientes', () => {
  assert.equal(terminalMode({stdinTTY:true, stdoutTTY:true, width:80, env:{CI:'1'}}).interactive, false);
  assert.equal(terminalMode({stdinTTY:false, stdoutTTY:true, width:80, env:{}}).interactive, false);
  const plain = terminalMode({stdinTTY:true, stdoutTTY:true, width:80, env:{NO_COLOR:'1'}});
  assert.equal(plain.color, false);
  assert.equal(plain.motion, false);
  const still = terminalMode({stdinTTY:true, stdoutTTY:true, width:80, env:{GOMEMORY_NO_MOTION:'1'}});
  assert.equal(still.color, true);
  assert.equal(still.motion, false);
});

test('NDJSON falla cerrado ante líneas en blanco igual que Go', () => {
  const complete='{"contract_version":1,"type":"complete","status":"ok","exit_code":0}\n';
  for (const blank of ['\n','\r\n',' \t\n']) {
    for (const input of [blank+complete,complete+blank,complete+' ']) {
      const stream=new EventStream(()=>{});
      assert.throws(()=>{stream.push(Buffer.from(input));stream.finish(0)},/blanco|cierre|truncado/);
    }
  }
});
