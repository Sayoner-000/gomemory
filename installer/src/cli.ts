#!/usr/bin/env node
import * as p from '@clack/prompts';
import pc from 'picocolors';
import { spawn } from 'node:child_process';
import { stat } from 'node:fs/promises';
import { resolve, dirname, delimiter, join } from 'node:path';
import { createInterface } from 'node:readline';
import { agents, EventStream, parseOptions, type Agent, type Options } from './core.js';
import { obtainBinary } from './bootstrap.js';
import { ink, theme, welcome, reveal, processMode } from './visual.js';

const abort = new AbortController();
const onSignal = () => abort.abort();
process.once('SIGINT', onSignal);
process.once('SIGTERM', onSignal);

function selected<T>(value:T): Exclude<T, symbol> {
  if (p.isCancel(value)) {abort.abort(); throw new Error('Instalación cancelada; no se configuró el proyecto')}
  return value as Exclude<T, symbol>;
}

async function configure(options:Options, interactive:boolean): Promise<Options> {
  if (!interactive || options.yes) return options;
  const chosen = options.agentsExplicit ? options.agents : selected(await p.multiselect<Agent>({
    message:'¿Qué agentes quieres configurar?', required:false, options:agents.map(agent => ({value:agent,label:agent})), signal:abort.signal
  }));
  const globalAllowed = chosen.every(agent => ['opencode','claude','codex'].includes(agent));
  const scope = selected(await p.select<'project'|'global'>({message:'Alcance de la integración', initialValue:options.scope,
    options:globalAllowed ? [{value:'project',label:'Este proyecto'},{value:'global',label:'Usuario global'}] : [{value:'project',label:'Este proyecto'}], signal:abort.signal}));
  if (!selected(await p.confirm({message:`Configurar ${resolve(options.target)} · ${scope} · ${chosen.join(', ') || 'sin agentes'}?`, signal:abort.signal}))) throw new Error('Instalación cancelada');
  return {...options, agents:chosen, agentsExplicit:true, scope};
}

async function run(): Promise<void> {
  let options = parseOptions(process.argv.slice(2));
  if (options.help) {
    console.log('goMemory · Instalador opcional\n\nUso: gomemory-install [directorio] [opciones]\n\nOPCIONES\n  --agents opencode,claude,cursor,codex,windsurf,cline\n  --scope project|global\n  --binary /ruta/mem      Usar un binario local compatible con --events\n  --version vX.Y.Z       Elegir release (default: latest)\n  --bin-dir /ruta        Destino del binario (default: ~/.local/bin)\n  --yes, -y              Sin preguntas\n  --no-motion            Sin animaciones\n  --help, -h             Mostrar ayuda');
    return;
  }
  const mode = processMode(options.noMotion);
  if (!(await stat(resolve(options.target))).isDirectory()) throw new Error('El proyecto debe ser un directorio existente');
  if (mode.interactive) {
    p.intro(welcome(process.stdout.columns ?? 80, process.env, mode.color));
    await reveal('Memoria que acompaña a tu código.\n', text => process.stdout.write(text), mode.motion, abort.signal);
  }
  options = await configure(options, mode.interactive);
  if (abort.signal.aborted) throw new Error('Instalación cancelada');
  const spinner = mode.motion ? p.spinner() : undefined;
  const message = (text:string) => {
    if (spinner) spinner.message(text);
    else console.log(`› ${text}`);
  };
  spinner?.start('Preparando goMemory');
  try {
    const binary = await obtainBinary(options, message, abort.signal);
    if (abort.signal.aborted) throw new Error('Instalación cancelada');
    const args = ['install', resolve(options.target), '--yes', '--events', '--scope', options.scope];
    if (options.agentsExplicit) args.push('--agents', options.agents.join(',') || 'none');
    const palette = theme(process.env);
    const stderr:string[] = [];
    const stream = new EventStream(event => {
      if (event.type === 'start') message('Configurando proyecto');
      if (event.type === 'step') {
        spinner?.stop('Paso completado');
        const symbol = event.status === 'ok' ? '✓' : event.status === 'warn' ? '⚠' : '✗';
        const color = event.status === 'ok' ? palette.secondary : event.status === 'warn' ? palette.warning : palette.error;
        console.log(`${ink(symbol,color,mode.color)} ${event.name}${event.detail ? ': '+event.detail : ''}`);
        if (event.manual) console.log(`  → ${event.manual}`);
        spinner?.start('Configurando proyecto');
      }
    });
    // Esperar close evita anunciar cancelación mientras el puente todavía
    // está deteniendo al instalador real y a sus descendientes.
    const child = spawn(binary, args, {stdio:['ignore','pipe','pipe']});
    let stopRequested=false;
    let stopTask:Promise<void> | undefined;
    const stop = () => {
      if (stopRequested || child.exitCode !== null || child.signalCode !== null || !child.pid) return;
      stopRequested=true;
      if (process.platform === 'win32') {
        const killer=spawn(join(process.env.SystemRoot ?? 'C:\\Windows','System32','taskkill.exe'),['/PID',String(child.pid),'/T','/F'],{stdio:'ignore'});
        stopTask=new Promise<void>((resolveStop,rejectStop)=>{killer.once('error',rejectStop);killer.once('close',code=>code===0 ? resolveStop() : rejectStop(new Error(`No se pudo detener la instalación (${code})`)))});
        // Registrar el rechazo hasta que el cierre del proceso permita esperar.
        void stopTask.catch(()=>{});
      } else child.kill('SIGTERM');
    };
    abort.signal.addEventListener('abort',stop,{once:true});
    if (abort.signal.aborted) stop();
    let protocolError:Error | undefined;
    child.stdout.on('data', (chunk:Buffer) => {try {stream.push(chunk)} catch(error) {protocolError=error as Error; stop();}});
    const lines = createInterface({input:child.stderr});
    lines.on('line', line => {stderr.push(line); if (stderr.length > 20) stderr.shift()});
    let spawnError:Error | undefined;
    let code:number;
    try {
      code = await new Promise<number>((resolveCode) => {
        child.once('error', error => {spawnError=error});
        child.once('close', code => resolveCode(code ?? 1));
      });
      if (stopTask) await stopTask;
    } finally {abort.signal.removeEventListener('abort',stop);lines.close()}
    if (abort.signal.aborted) throw new Error('Instalación cancelada');
    if (spawnError) throw spawnError;
    if (protocolError) throw protocolError;
    let result;
    try {result = stream.finish(code)} catch(error) {throw new Error(`${String(error)}\n${stderr.join('\n')}`)}
    spinner?.stop(result.status === 'warn' ? 'Instalación con avisos' : 'Instalación completada');
    const finish = result.status === 'warn' ? 'Revisa los avisos y los comandos indicados.' : 'goMemory listo. Ejecuta mem tui en el proyecto.';
    if (mode.interactive) p.outro(finish); else console.log(finish);
    const pathDirs = (process.env.PATH ?? '').split(delimiter).map(path => resolve(path));
    if (!pathDirs.includes(dirname(binary))) console.log(`Añade ${dirname(binary)} al PATH para ejecutar mem desde cualquier terminal.`);
  } catch(error) {
    spinner?.stop('Instalación interrumpida');
    throw error;
  }
}

run().catch(error => {
  const text = error instanceof Error ? error.message : String(error);
  console.error(pc.red(`✗ ${text}`));
  process.exitCode = abort.signal.aborted ? 130 : 1;
}).finally(() => {process.removeListener('SIGINT',onSignal); process.removeListener('SIGTERM',onSignal)});
