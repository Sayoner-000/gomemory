import { mkdir, copyFile } from 'node:fs/promises';
const target = new URL('../dist/assets/', import.meta.url);
await mkdir(target, { recursive: true });
for (const file of ['console-themes.json', 'gomemory-terminal.txt']) {
  await copyFile(new URL(`../../assets/${file}`, import.meta.url), new URL(file, target));
}
