import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { stripVTControlCharacters } from 'node:util';
import { welcome, theme, ink, reveal } from '../dist/visual.js';

test('logo fiel al asset compartido y gradiente dark azul/cian/violeta', async () => {
  const mark = (await readFile(new URL('../../assets/gomemory-terminal.txt',import.meta.url),'utf8')).trimEnd();
  const text = welcome(80,{GOMEMORY_THEME:'dark'},true);
  assert.ok(stripVTControlCharacters(text).includes(mark));
  for (const color of ['#2c8ff0','#06c1ee','#22e3f7','#6b56d4']) assert.ok(text.includes(ink('',color,true).split('\x1b[0m')[0]),`falta ${color}`);
  assert.equal(stripVTControlCharacters(welcome(40,{},false)),'goMemory · Instalar');
});

test('la revelación sin movimiento preserva el texto completo de inmediato', async () => {
  const calls=[];
  await reveal('goMemory · 界',text=>calls.push(text),false,new AbortController().signal);
  assert.deepEqual(calls,['goMemory · 界']);
  assert.equal(theme({GOMEMORY_THEME:'matrix'}).name,'matrix');
});

test('C-002: mismos casos de resolución de tema que Go', async () => {
  const cases=JSON.parse(await readFile(new URL('../../assets/theme-resolution-cases.json',import.meta.url),'utf8'));
  for (const example of cases) assert.equal(theme(example.env).name,example.expected,example.name);
});
