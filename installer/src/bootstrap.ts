import { access, chmod, copyFile, mkdtemp, rename, rm, stat, writeFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { homedir, tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { x } from 'tar';
import { unzipSync } from 'fflate';
import { assetName, verifyChecksum, type Options } from './core.js';

const repository = 'Sayoner-000/gomemory';

// Topes de tamaño (un release ronda los 8 MB). Se aplican antes de verificar
// el checksum, que solo puede comprobarse con el archivo completo en memoria.
const maxMetadataBytes = 1 << 20;
const maxArchiveBytes = 128 << 20;
const maxBinaryBytes = 256 << 20;

// download rechaza un Content-Length mayor que el tope sin leer el cuerpo y
// corta la lectura en cuanto lo acumulado lo supera, por si no se declara.
export async function download(url: string, limit: number, signal?: AbortSignal): Promise<Uint8Array> {
  const timeout = AbortSignal.timeout(60_000);
  const response = await fetch(url, { signal: signal ? AbortSignal.any([signal,timeout]) : timeout, headers: {'User-Agent':'gomemory-installer'} });
  if (!response.ok) throw new Error(`Descarga fallida (${response.status}): ${url}`);
  const tooLarge = () => new Error(`Descarga mayor que el máximo permitido (${limit} bytes): ${url}`);
  if (Number(response.headers.get('content-length')) > limit) {
    await response.body?.cancel();
    throw tooLarge();
  }
  if (!response.body) return new Uint8Array();
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > limit) {
      await reader.cancel();
      throw tooLarge();
    }
    chunks.push(value);
  }
  return Buffer.concat(chunks);
}

type PlaceOps = { rename: (from: string, to: string) => Promise<void> };

// placeBinary deja el binario preparado en su destino. En Windows un mem.exe en
// ejecución (p. ej. el servidor MCP) no puede reemplazarse, pero sí apartarse:
// se renombra el actual, se coloca el nuevo y, si algo falla, se restaura. El
// apartado se borra si ya no está en uso; si lo está, queda y no estorba.
export async function placeBinary(staged: string, destination: string, platform: string, ops: PlaceOps = {rename}): Promise<void> {
  if (platform !== 'win32') {
    await ops.rename(staged, destination);
    return;
  }
  let aside: string | undefined;
  try {
    await access(destination);
    aside = `${destination}.old-${randomUUID()}`;
    await ops.rename(destination, aside);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
  }
  try {
    await ops.rename(staged, destination);
  } catch (error) {
    if (aside) await ops.rename(aside, destination);
    throw error;
  }
  if (aside) await rm(aside, {force:true}).catch(() => undefined);
}

// La extracción queda limitada al ejecutable, después de verificar el archivo.
export async function extractBinary(archive: Uint8Array, platform: string, directory: string): Promise<string> {
  const name = platform === 'win32' ? 'mem.exe' : 'mem';
  const output = join(directory, name);
  if (platform === 'win32') {
    const contents = unzipSync(archive, {filter: entry => {
      if (entry.name === name && entry.originalSize > maxBinaryBytes) throw new Error('El ejecutable del release supera el tamaño máximo');
      return entry.name === name;
    }});
    const binary = contents[name];
    if (!binary) throw new Error('El release no contiene mem.exe');
    await writeFile(output, binary, {mode:0o755});
  } else {
    const file = join(directory, 'release.tar.gz');
    await writeFile(file, archive);
    let oversized = false;
    await x({file, cwd:directory, strict:true, filter: (path, entry) => {
      if (path !== name || !('type' in entry) || entry.type !== 'File') return false;
      oversized = entry.size > maxBinaryBytes;
      return !oversized;
    }});
    if (oversized) throw new Error('El ejecutable del release supera el tamaño máximo');
  }
  if (!(await stat(output)).isFile()) throw new Error('El release no contiene el ejecutable mem');
  await chmod(output, 0o755);
  return output;
}

export async function obtainBinary(options: Options, progress: (message:string) => void, signal?:AbortSignal): Promise<string> {
  signal?.throwIfAborted();
  if (options.binary) {
    const path = resolve(options.binary);
    await access(path, process.platform === 'win32' ? constants.F_OK : constants.X_OK);
    if (!(await stat(path)).isFile()) throw new Error('--binary debe apuntar a un ejecutable');
    return path;
  }
  const asset = assetName(process.platform, process.arch);
  const version = options.version === 'latest' ? 'latest' : options.version.replace(/^([^v])/, 'v$1');
  const endpoint = version === 'latest' ? 'latest' : `tags/${version}`;
  progress('Resolviendo release de goMemory');
  const metadata: unknown = JSON.parse(Buffer.from(await download(`https://api.github.com/repos/${repository}/releases/${endpoint}`,maxMetadataBytes,signal)).toString());
  if (!metadata || typeof metadata !== 'object' || !('tag_name' in metadata) || typeof metadata.tag_name !== 'string' || !/^v?\d+\.\d+\.\d+$/.test(metadata.tag_name)) throw new Error('Release estable inválido');
  const base = `https://github.com/${repository}/releases/download/${metadata.tag_name}`;
  progress(`Descargando ${asset} · ${metadata.tag_name}`);
  const [archive, checksums] = await Promise.all([download(`${base}/${asset}`,maxArchiveBytes,signal), download(`${base}/checksums.txt`,maxMetadataBytes,signal)]);
  verifyChecksum(archive, Buffer.from(checksums).toString(), asset);
  signal?.throwIfAborted();
  const temporary = await mkdtemp(join(tmpdir(), 'gomemory-'));
  const directory = resolve(options.binDir ?? join(homedir(), '.local', 'bin'));
  const { mkdir } = await import('node:fs/promises');
  let staged: string | undefined;
  try {
    const binary = await extractBinary(archive, process.platform, temporary);
    await mkdir(directory, {recursive:true});
    staged = join(directory, `.gomemory-${randomUUID()}`);
    await copyFile(binary, staged);
    await chmod(staged, 0o755);
    const destination = join(directory, process.platform === 'win32' ? 'mem.exe' : 'mem');
    await placeBinary(staged, destination, process.platform);
    staged = undefined;
    return destination;
  } finally {
    if (staged) await rm(staged, {force:true});
    await rm(temporary, {recursive:true, force:true});
  }
}
