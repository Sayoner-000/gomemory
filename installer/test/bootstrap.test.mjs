import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile, rm, mkdir, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { zipSync } from 'fflate';
import { c } from 'tar';
import { extractBinary, obtainBinary } from '../dist/bootstrap.js';
import { parseOptions, assetName } from '../dist/core.js';

test('ZIP y TAR solo extraen el ejecutable, sin archivos adicionales', async () => {
  const root = await mkdtemp(join(tmpdir(),'gomemory-extract-'));
  try {
    const zip = zipSync({'mem.exe':Buffer.from('binary'), '../fuera':Buffer.from('no')});
    const windows = join(root,'windows');
    await mkdir(windows);
    assert.equal(await readFile(await extractBinary(zip,'win32',windows),'utf8'),'binary');
    assert.deepEqual(await readdir(windows),['mem.exe']);
    const source = join(root,'source');
    const output = join(root,'output');
    await mkdir(source); await mkdir(output);
    await writeFile(join(source,'mem'),'native');
    await writeFile(join(source,'otro'),'ignore');
    const archive = join(root,'release.tar.gz');
    await c({gzip:true, file:archive, cwd:source},['mem','otro']);
    assert.equal(await readFile(await extractBinary(await readFile(archive),'darwin',output),'utf8'),'native');
    assert.deepEqual((await readdir(output)).sort(),['mem','release.tar.gz']);
  } finally {await rm(root,{recursive:true,force:true})}
});

test('un checksum incorrecto no sustituye el binario instalado', async t => {
  const root = await mkdtemp(join(tmpdir(),'gomemory-checksum-'));
  const name = process.platform === 'win32' ? 'mem.exe' : 'mem';
  const asset = assetName(process.platform,process.arch);
  await writeFile(join(root,name),'previous');
  t.mock.method(globalThis,'fetch', async url => {
    if (String(url).includes('api.github.com')) return new Response(JSON.stringify({tag_name:'v2.29.0'}));
    if (String(url).endsWith('checksums.txt')) return new Response('0'.repeat(64)+'  '+asset);
    return new Response('tampered');
  });
  try {
    await assert.rejects(obtainBinary(parseOptions(['--bin-dir',root]),()=>{}),/checksum/);
    assert.equal(await readFile(join(root,name),'utf8'),'previous');
    assert.deepEqual(await readdir(root),[name]);
  } finally {await rm(root,{recursive:true,force:true})}
});

test('cancelar antes de descargar no realiza peticiones ni escrituras', async t => {
  let calls = 0;
  t.mock.method(globalThis,'fetch', async () => {calls++; throw new Error('request')});
  const controller = new AbortController(); controller.abort();
  await assert.rejects(obtainBinary(parseOptions([]),()=>{},controller.signal));
  assert.equal(calls,0);
});

// S-001: en Windows un mem.exe en ejecución no puede reemplazarse, pero sí
// apartarse con otro nombre. El rename inyectado reproduce ese bloqueo.
const lockedRename = (realRename, destination) => async (from, to) => {
  const { existsSync } = await import('node:fs');
  if (to === destination && existsSync(destination)) throw Object.assign(new Error('EBUSY: archivo en uso'), {code:'EBUSY'});
  return realRename(from, to);
};

test('Windows: reemplaza un mem.exe en uso apartándolo antes', async () => {
  const { placeBinary } = await import('../dist/bootstrap.js');
  const { rename } = await import('node:fs/promises');
  const dir = await mkdtemp(join(tmpdir(),'gomemory-place-'));
  const destination = join(dir,'mem.exe'), staged = join(dir,'.gomemory-nuevo');
  await writeFile(destination,'viejo'); await writeFile(staged,'nuevo');
  await placeBinary(staged, destination, 'win32', {rename: lockedRename(rename, destination)});
  assert.equal(await readFile(destination,'utf8'), 'nuevo');
  assert.deepEqual((await readdir(dir)).sort(), ['mem.exe']);
  await rm(dir,{recursive:true,force:true});
});

test('Windows: si falla la colocación, restaura el binario anterior', async () => {
  const { placeBinary } = await import('../dist/bootstrap.js');
  const { rename } = await import('node:fs/promises');
  const dir = await mkdtemp(join(tmpdir(),'gomemory-place-'));
  const destination = join(dir,'mem.exe'), staged = join(dir,'.gomemory-nuevo');
  await writeFile(destination,'viejo'); await writeFile(staged,'nuevo');
  const failing = async (from, to) => { if (from === staged) throw new Error('disco lleno'); return rename(from, to); };
  await assert.rejects(placeBinary(staged, destination, 'win32', {rename: failing}), /disco lleno/);
  assert.equal(await readFile(destination,'utf8'), 'viejo');
  await rm(dir,{recursive:true,force:true});
});

test('Unix: reemplazo atómico directo', async () => {
  const { placeBinary } = await import('../dist/bootstrap.js');
  const dir = await mkdtemp(join(tmpdir(),'gomemory-place-'));
  const destination = join(dir,'mem'), staged = join(dir,'.gomemory-nuevo');
  await writeFile(destination,'viejo'); await writeFile(staged,'nuevo');
  await placeBinary(staged, destination, 'darwin');
  assert.equal(await readFile(destination,'utf8'), 'nuevo');
  assert.deepEqual(await readdir(dir), ['mem']);
  await rm(dir,{recursive:true,force:true});
});
