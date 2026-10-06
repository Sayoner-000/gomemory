import { createHash } from 'node:crypto';
import { StringDecoder } from 'node:string_decoder';
import { parseArgs } from 'node:util';

export const agents = ['opencode', 'claude', 'cursor', 'codex', 'windsurf', 'cline'] as const;
export type Agent = typeof agents[number];
export interface Options {
  target: string; yes: boolean; noMotion: boolean; help: boolean; agentsExplicit: boolean;
  agents: Agent[]; scope: 'project' | 'global'; binary: string | undefined;
  version: string; binDir: string | undefined;
}

export function parseOptions(args: string[]): Options {
  const { values: v, positionals } = parseArgs({args, allowPositionals: true, options: {
    yes: {type:'boolean', short:'y'}, 'no-motion': {type:'boolean'}, help: {type:'boolean', short:'h'},
    agents: {type:'string'}, scope: {type:'string'}, binary: {type:'string'},
    version: {type:'string'}, 'bin-dir': {type:'string'}
  }});
  if (positionals.length > 1) throw new Error('Indica un solo directorio de proyecto.');
  const selected = v.agents && v.agents !== 'none' ? v.agents.split(',') : [];
  if (v.agents === '') throw new Error('--agents está vacío; usa none para elegir ninguno');
  for (const agent of selected) if (!agents.includes(agent as Agent)) throw new Error(`agente no soportado: ${agent}`);
  const scope = v.scope ?? 'project';
  if (scope !== 'project' && scope !== 'global') throw new Error('--scope admite project o global');
  if (scope === 'global' && selected.some(a => !['opencode','claude','codex'].includes(a))) throw new Error('Este agente solo admite alcance de proyecto');
  const version = v.version ?? 'latest';
  if (version !== 'latest' && !/^v?\d+\.\d+\.\d+$/.test(version)) throw new Error('versión inválida; usa vX.Y.Z');
  return {target: positionals[0] ?? '.', yes: v.yes ?? false, noMotion: v['no-motion'] ?? false,
    help: v.help ?? false, agentsExplicit: v.agents !== undefined, agents: [...new Set(selected)] as Agent[], scope, binary:v.binary,
    version, binDir:v['bin-dir']};
}

export function assetName(platform: string, arch: string): string {
  const os = platform === 'win32' ? 'windows' : platform;
  const cpu = arch === 'x64' ? 'amd64' : arch;
  if (!['linux','darwin','windows'].includes(os) || !['amd64','arm64'].includes(cpu) || (os === 'windows' && cpu === 'arm64')) {
    throw new Error(`Plataforma no soportada: ${platform}/${arch}`);
  }
  return `mem_${os}_${cpu}.${os === 'windows' ? 'zip' : 'tar.gz'}`;
}

export function verifyChecksum(data: Uint8Array, manifest: string, asset: string): void {
  const entry = manifest.split(/\r?\n/).map(line => line.trim().split(/\s+/)).find(parts => parts[1]?.replace(/^\*/, '') === asset);
  const actual = createHash('sha256').update(data).digest('hex');
  if (!entry || entry[0] !== actual) throw new Error(`checksum inválido o ausente para ${asset}`);
}

export function terminalMode(input: {stdinTTY:boolean; stdoutTTY:boolean; width:number; env:NodeJS.ProcessEnv}) {
  const interactive = input.stdinTTY && input.stdoutTTY && !input.env.CI;
  const color = input.stdoutTTY && !input.env.CI && !('NO_COLOR' in input.env) && input.env.TERM !== 'dumb' && input.width >= 40;
  return {interactive, color, motion: color && !input.env.GOMEMORY_NO_MOTION && !input.env.GOMEMORY_REDUCED_MOTION && !input.env.ACCESSIBLE};
}

export interface InstallEvent {
  contract_version: 1; type: 'start' | 'step' | 'complete';
  name?: string; status?: 'ok' | 'warn' | 'fail'; detail?: string; manual?: string; exit_code?: number;
}

function decodeEvent(line: string): InstallEvent {
  const event: unknown = JSON.parse(line);
  if (!event || typeof event !== 'object') throw new Error('Evento fuera del contrato de instalación');
  const e = event as Record<string, unknown>;
  if (e.contract_version !== 1 || !['start','step','complete'].includes(String(e.type))) throw new Error('Versión de contrato de instalación no soportada');
  if (e.type !== 'start' && !['ok','warn','fail'].includes(String(e.status))) throw new Error('Estado fuera del contrato');
  for (const key of ['name','detail','manual']) if (e[key] !== undefined && typeof e[key] !== 'string') throw new Error('Texto fuera del contrato');
  if (e.type === 'step' && !e.name) throw new Error('Paso sin nombre');
  if (e.type === 'complete' && (!Number.isInteger(e.exit_code) || Number(e.exit_code) < 0)) throw new Error('Código de salida fuera del contrato');
  return e as unknown as InstallEvent;
}

export class EventStream {
  private decoder = new StringDecoder('utf8');
  private pending = '';
  private complete: InstallEvent | undefined;
  private hasWarnings = false;
  constructor(private readonly receive: (event: InstallEvent) => void) {}
  push(chunk: Uint8Array): void {
    this.pending += this.decoder.write(Buffer.from(chunk));
    if (this.pending.length > 4*1024*1024) throw new Error('Evento demasiado largo');
    let index: number;
    while ((index = this.pending.indexOf('\n')) >= 0) {
      const line = this.pending.slice(0, index).trim();
      this.pending = this.pending.slice(index+1);
      if (!line) throw new Error('Línea en blanco fuera del contrato NDJSON');
      if (this.complete) throw new Error('Evento recibido después del cierre');
      const event = decodeEvent(line);
      if (event.type === 'step' && event.status !== 'ok') this.hasWarnings = true;
      if (event.type === 'complete') this.complete = event;
      this.receive(event);
    }
  }
  finish(exitCode: number): InstallEvent {
    this.pending += this.decoder.end();
    if (this.pending.length) throw new Error('Evento truncado: falta salto de línea de cierre');
    if (!this.complete) throw new Error('Sin cierre de instalación; el binario debe soportar mem install --events');
    if (this.hasWarnings && this.complete.status === 'ok') throw new Error('El cierre oculta avisos de instalación');
    if (this.complete.status === 'fail') throw new Error('Instalación fallida según el motor');
    if (exitCode !== this.complete.exit_code || exitCode !== 0) throw new Error(`Instalación fallida: código de salida ${exitCode}`);
    return this.complete;
  }
}
