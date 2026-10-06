import { readFileSync } from 'node:fs';
import { stripVTControlCharacters } from 'node:util';
import { terminalMode } from './core.js';

interface Palette {name:string; primary:string; secondary:string; text:string; muted:string; background:string; warning:string; error:string; logo:string[]}
const palettes = JSON.parse(readFileSync(new URL('./assets/console-themes.json', import.meta.url), 'utf8')) as Record<string, Palette>;
const logo = readFileSync(new URL('./assets/gomemory-terminal.txt', import.meta.url), 'utf8').trimEnd();

export function theme(env:NodeJS.ProcessEnv): Palette {
  const rawBackground=env.COLORFGBG?.split(';').at(-1) ?? '';
  const background=/^[+-]?\d+$/.test(rawBackground) ? Number(rawBackground) : Number.NaN;
  const inferred = [7,15].includes(background) ? 'light' : 'dark';
  const requested=(env.GOMEMORY_THEME ?? '').trim().toLowerCase();
  const selected=['dark','light','matrix'].includes(requested) ? requested : inferred;
  return palettes[selected]!;
}

export function ink(text:string, hex:string, enabled:boolean): string {
  if (!enabled) return text;
  const n = Number.parseInt(hex.slice(1), 16);
  return `\x1b[38;2;${n>>16};${(n>>8)&255};${n&255}m${text}\x1b[0m`;
}

export function welcome(width:number, env:NodeJS.ProcessEnv, color:boolean): string {
  const p = theme(env);
  const name = ink('go',p.secondary,color)+ink('Memory',p.text,color);
  if (width < 78 || !color) return `${name} · Instalar`;
  const lines=logo.split('\n');
  return lines.map((line,index) => ink(line,p.logo[Math.floor(index*p.logo.length/lines.length)] ?? p.primary,color)).join('\n')+`\n\n${name} · Memoria persistente para agentes de código`;
}

// La revelación es breve y cancelable; los informes y errores siempre son inmediatos.
export async function reveal(text:string, write:(text:string)=>void, motion:boolean, signal:AbortSignal): Promise<void> {
  if (!motion) { write(text); return; }
  for (const character of Array.from(stripVTControlCharacters(text))) {
    if (signal.aborted) throw new Error('Instalación cancelada');
    write(character);
    await new Promise<void>(resolve => setTimeout(resolve, 6));
  }
}

export function processMode(noMotion:boolean) {
  const mode = terminalMode({stdinTTY:!!process.stdin.isTTY, stdoutTTY:!!process.stdout.isTTY, width:process.stdout.columns ?? 80, env:process.env});
  return {...mode, motion:mode.motion && !noMotion};
}
