import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, writeFile, chmod, rm, access } from 'node:fs/promises';
import { existsSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

test('el cliente espera close y la limpieza antes de anunciar Cancelado', {skip:process.platform==='win32'}, async () => {
  const root=await mkdtemp(join(tmpdir(),'gomemory-client-cancel-'));
  const ready=join(root,'ready'), closed=join(root,'closed'), pidFile=join(root,'pid');
  const bridge=join(root,'bridge');
  await writeFile(bridge,`#!${process.execPath}
import { writeFileSync } from 'node:fs';
process.on('SIGTERM',()=>setTimeout(()=>{writeFileSync(${JSON.stringify(closed)},'closed');process.exit(130)},250));
writeFileSync(${JSON.stringify(pidFile)},String(process.pid));
console.log(JSON.stringify({contract_version:1,type:'start',name:'Instalación'}));
writeFileSync(${JSON.stringify(ready)},'ready');
setInterval(()=>{},1000);
`);
  await chmod(bridge,0o755);
  const child=spawn(process.execPath,[fileURLToPath(new URL('../dist/cli.js',import.meta.url)),root,'--binary',bridge,'--yes','--agents','none','--no-motion'],{env:{...process.env,CI:'1',NO_COLOR:'1'},stdio:['ignore','pipe','pipe']});
  let early=false, text='';
  const record=chunk=>{const value=chunk.toString();text+=value;if (/cancelad|aborted|interrumpida/i.test(value) && !existsSync(closed)) early=true};
  child.stdout.on('data',record);child.stderr.on('data',record);
  const ending=new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',resolve)});
  try {
    const deadline=Date.now()+5000;
    while (!existsSync(ready)) {
      if (Date.now()>deadline || child.exitCode!==null) throw new Error('puente no inició: '+text);
      await new Promise(resolve=>setTimeout(resolve,10));
    }
    child.kill('SIGTERM');
    assert.equal(await ending,130);
    await access(closed);
    assert.equal(early,false,'se anunció cancelación antes de esperar al puente');
    assert.match(text,/Instalación cancelada/);
  } finally {
    if (child.exitCode===null) child.kill('SIGKILL');
    await ending.catch(()=>{});
    if (existsSync(pidFile)) {try {process.kill(Number(readFileSync(pidFile,'utf8')),'SIGKILL')} catch(error) {if(error.code!=='ESRCH') throw error}}
    await rm(root,{recursive:true,force:true});
  }
});
