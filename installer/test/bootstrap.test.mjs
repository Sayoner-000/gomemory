import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile, rm, mkdir, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { zipSync } from 'fflate';
import { c } from 'tar';
import { download, extractBinary, obtainBinary } from '../dist/bootstrap.js';
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

// H-02 (auditoría 2026-10-17, ronda 2): el tamaño se acota antes de verificar
// el checksum, sin materializar una descarga sobredimensionada.
test('un Content-Length mayor que el tope falla sin leer el cuerpo ni escribir', async t => {
  const root = await mkdtemp(join(tmpdir(),'gomemory-size-'));
  let pulled = 0;
  t.mock.method(globalThis,'fetch', async url => {
    if (String(url).includes('api.github.com')) return new Response(JSON.stringify({tag_name:'v2.29.0'}));
    const body = new ReadableStream({pull(controller) {pulled++; controller.enqueue(new Uint8Array(1024))}});
    return new Response(body, {headers:{'content-length':String(1 << 30)}});
  });
  try {
    await assert.rejects(obtainBinary(parseOptions(['--bin-dir',root]),()=>{}),/máximo permitido/);
    // Archivo y checksums se piden en paralelo; cada stream precarga un bloque
    // al crearse. Más de eso sería leer el cuerpo pese al Content-Length.
    assert.ok(pulled <= 2, `leyó ${pulled} bloques pese al Content-Length`);
    assert.deepEqual(await readdir(root),[]);
  } finally {await rm(root,{recursive:true,force:true})}
});

test('sin Content-Length la lectura se corta al superar el tope', async t => {
  let pulled = 0;
  t.mock.method(globalThis,'fetch', async () => new Response(new ReadableStream({pull(controller) {pulled++; controller.enqueue(new Uint8Array(1024))}})));
  await assert.rejects(download('https://example.invalid/x',4096),/máximo permitido/);
  assert.ok(pulled < 10, `siguió leyendo: ${pulled} bloques`);
  t.mock.method(globalThis,'fetch', async () => new Response('dentro'));
  assert.equal(Buffer.from(await download('https://example.invalid/y',4096)).toString(),'dentro');
});

test('un ejecutable declarado por encima del tope no se extrae', async () => {
  const root = await mkdtemp(join(tmpdir(),'gomemory-bomb-'));
  try {
    const zip = zipSync({'mem.exe':Buffer.from('binary')});
    // originalSize declarado en la cabecera central (offset 24 desde la firma).
    const central = Buffer.from(zip).lastIndexOf(Buffer.from([0x50,0x4b,0x01,0x02]));
    Buffer.from(zip.buffer, zip.byteOffset).writeUInt32LE(0x7fffffff, central + 24);
    await assert.rejects(extractBinary(zip,'win32',root),/tamaño máximo/);
    assert.deepEqual(await readdir(root),[]);
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
