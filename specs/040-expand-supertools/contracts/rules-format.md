# Contrato: reglas `.gomemory/rules/*.md`

## Formato

```markdown
---
name: no-prod-db
event: bash            # bash | file | prompt | stop | any
match: regex           # contains (por defecto) | regex (RE2)
pattern: 'psql .*prod'
action: block          # warn | block
enabled: true          # opcional, true por defecto
overrides: ''          # opcional: nombre de una regla de serie que esta sustituye
---
No conectes a la base de datos de producción desde el agente. Usa la réplica de staging.
```

## Carga

- Solo se leen archivos regulares `*.md` del primer nivel; los enlaces simbólicos se ignoran. Se cargan como máximo 64 archivos, en orden lexicográfico.
- Cada archivo tiene un tope de 16 KiB y cada patrón de 512 caracteres. El cuerpo se sanea con `StripTerminalControls` y se recorta a 2 KiB.
- Un archivo inválido produce un `RuleLoadError` y no detiene la carga del resto.
- Caché `.memory/.expand-rules-cache.json` con clave (nombre, tamaño, mtime) por archivo. La caché guarda reglas ya parseadas, validadas y saneadas, no expresiones compiladas: RE2 se compila en cada proceso del hook, que es barato con 64 reglas de 512 caracteres como máximo; T074 lo confirma (3.ª pasada, A2). Si cualquier entrada cambia, se recompila todo; si la caché está corrupta, se recompila sin error.

## Regla de serie `destructive-shell`

| Comando | Decisión |
|---|---|
| `rm -rf x`, `rm -fr x`, `rm -r -f x`, `rm --recursive --force x`, `sudo rm -rf /` | block |
| `rm -r x` (sin force), `rm x` | allow |
| `git reset --hard`, `git reset --hard HEAD~1` | block |
| `git reset --soft HEAD~1` | allow |
| `git push --force`, `git push -f origin main` | block |
| `git push --force-with-lease` | warn |
| `git push --mirror`, `git push origin +main` | block (el refspec `+` fuerza la actualización) |
| `git push origin :rama`, `git push origin --delete rama` | warn (borra la rama remota) |
| `git checkout -- .`, `git checkout .`, `git checkout -- ./`, `git restore .`, `git restore ./`, `git restore --worktree .`, `git checkout -- :/` | block |
| `git restore --staged .`, `git restore -S .` (solo deshace el indexado) | allow |
| `git checkout -- file.go` | allow |
| `git clean -fd`, `git clean -fdx`, `git clean -f` | block (`-n`/`--dry-run` permite) |
| `find x -delete` | block |
| `trap 'rm -rf x' EXIT` | block (la acción se evalúa como línea) |
| `R=rm; $R -rf x`, `$(echo rm) -rf x`, `ARG='rm -rf x'; bash -c "$ARG"` | block (asignaciones literales de la línea y `echo`/`printf` en sustituciones) |
| `echo "rm -rf /"`, `grep -- -rf file` | allow |
| `ls && rm -rf build` | block (se evalúa cada subcomando) |
| `/bin/rm -rf /`, `sudo -u root rm -rf /`, `env rm -rf /`, `command rm -rf x`, `nohup nice -n 5 rm -rf x` | block (se normaliza la ruta del ejecutable y se saltan los lanzadores con sus opciones) |
| `timeout 5 rm -rf x`, `xargs rm -rf`, `find x -exec rm -rf {} +`, `pkexec rm -rf x`, `su -c 'rm -rf x'`, `runuser -u app -- rm -rf x` | block (más lanzadores) |
| `sh -c "rm -rf /"`, `bash -lc 'ls && git reset --hard'`, `eval 'rm -rf x'` | block (`sh\|bash\|zsh\|dash\|ksh\|csh\|tcsh\|fish -c` y `eval` se evalúan como línea anidada; más de 4 niveles se bloquean) |
| `echo $(rm -rf x)`, `` echo `rm -rf x` ``, `diff <(rm -rf x) y`, `(cd x && rm -rf y)` | block (sustituciones y subshells) |
| `bash <<EOF` con `rm -rf x`, `bash <<< 'rm -rf x'`, `echo 'rm -rf x' \| sh` | block (script por la entrada estándar del shell) |
| `command -v git` | allow (solo consulta) |
| `sh -c "echo rm -rf /"`, `sh script.sh` | allow |
| `true & rm -rf build` | block (`&` de segundo plano separa comandos) |
| `go test ./... 2>&1`, `cmd &>log` | allow (`&` de redirección no separa) |

Es un guardarraíl ante comandos propuestos por el agente, no un sandbox: no interpreta variables del entorno ni alias, no lee scripts que se ejecuten desde un archivo (`bash script.sh`, `sh < script`, `. script`, `source script`) ni analiza código de otros intérpretes (`python -c`, `node -e`), y no cubre `flock` ni `watch`.

Se anula con `overrides: destructive-shell` en una regla propia con `enabled: false`, o con `mem expand off rules`.

## Mensaje de bloqueo

`gomemory expand · regla <name>: <mensaje>. Si es intencionado, ejecútalo tú en tu terminal o desactiva la regla (mem expand off rules).`
