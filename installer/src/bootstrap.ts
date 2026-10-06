import { access, chmod, copyFile, mkdtemp, rename, rm, stat, writeFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { homedir, tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { randomUUID } from 'node:crypto';
import { x } from 'tar';
import { unzipSync } from 'fflate';
import { assetName, verifyChecksum, type Options } from './core.js';

const repository = 'Sayoner-000/gomemory';

async function get(url: string, signal?: AbortSignal): Promise<Uint8Array> {
  const timeout = AbortSignal.timeout(60_000);
  const response = await fetch(url, { signal: signal ? AbortSignal.any([signal,timeout]) : timeout, headers: {'User-Agent':'gomemory-installer'} });
  if (!response.ok) throw new Error(`Descarga fallida (${response.status}): ${url}`);
  return new Uint8Array(await response.arrayBuffer());
}

// La extracción queda limitada al ejecutable, después de verificar el archivo.
export async function extractBinary(archive: Uint8Array, platform: string, directory: string): Promise<string> {
  const name = platform === 'win32' ? 'mem.exe' : 'mem';
  const output = join(directory, name);
  if (platform === 'win32') {
    const contents = unzipSync(archive, {filter: entry => entry.name === name});
    const binary = contents[name];
    if (!binary) throw new Error('El release no contiene mem.exe');
    await writeFile(output, binary, {mode:0o755});
  } else {
    const file = join(directory, 'release.tar.gz');
    await writeFile(file, archive);
    await x({file, cwd:directory, strict:true, filter: (path, entry) => path === name && 'type' in entry && entry.type === 'File'});
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
  const metadata: unknown = JSON.parse(Buffer.from(await get(`https://api.github.com/repos/${repository}/releases/${endpoint}`,signal)).toString());
  if (!metadata || typeof metadata !== 'object' || !('tag_name' in metadata) || typeof metadata.tag_name !== 'string' || !/^v?\d+\.\d+\.\d+$/.test(metadata.tag_name)) throw new Error('Release estable inválido');
  const base = `https://github.com/${repository}/releases/download/${metadata.tag_name}`;
  progress(`Descargando ${asset} · ${metadata.tag_name}`);
  const [archive, checksums] = await Promise.all([get(`${base}/${asset}`,signal), get(`${base}/checksums.txt`,signal)]);
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
    await rename(staged, destination);
    return destination;
  } finally {
    if (staged) await rm(staged, {force:true});
    await rm(temporary, {recursive:true, force:true});
  }
}
