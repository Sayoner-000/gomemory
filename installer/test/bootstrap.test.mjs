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
