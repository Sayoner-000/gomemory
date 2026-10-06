# Instalador de terminal de goMemory

Entrada opcional en TypeScript para configurar goMemory con preguntas en línea,
progreso y un resumen de resultados. La configuración del proyecto la ejecuta
el binario Go mediante `mem install --events`.

Requiere **Node.js 22.20 o superior**. Los instaladores Bash y PowerShell siguen
disponibles para instalar el binario sin Node.js.

## Ejecutar desde este repositorio

Desde la raíz de goMemory:

```sh
go build -o ./mem ./infrastructure
npm --prefix installer ci
npm --prefix installer run build
node installer/dist/cli.js /ruta/al/proyecto --binary ./mem
```

Para una ejecución sin preguntas:

```sh
node installer/dist/cli.js /ruta/al/proyecto --binary ./mem \
  --agents opencode,claude --scope project --yes --no-motion
```

Usa `--agents none` para configurar la memoria sin añadir agentes. Si se omite
`--agents` en modo no interactivo, el binario conserva su selección nativa:
agentes guardados o detectados.

## Obtener el binario desde un release

Sin `--binary`, el instalador consulta el release estable de GitHub, descarga el
archivo de la plataforma y verifica SHA-256 contra `checksums.txt` antes de
extraer el ejecutable. Lo instala en `~/.local/bin`, o en `--bin-dir`.

```sh
node installer/dist/cli.js /ruta/al/proyecto --version vX.Y.Z --yes
```

El release elegido debe incluir `mem install --events`. Un binario anterior que
no implemente ese contrato produce un error; no se anuncia una instalación
completa. El paquete tiene `private: true` y todavía no se publica en npm.

## Presentación y cancelación

- `GOMEMORY_THEME=dark|light|matrix` selecciona la paleta compartida.
- `--no-motion`, `GOMEMORY_NO_MOTION=1` o `GOMEMORY_REDUCED_MOTION=1` desactivan
  el movimiento sin quitar los colores.
- `NO_COLOR` y `TERM=dumb` desactivan los colores y el movimiento.
- CI y entrada/salida redirigida omiten las preguntas. Esc o Ctrl+C cancelan las
  preguntas antes de configurar el proyecto.
- Los avisos conservan su detalle y el comando manual indicado por Go.

Durante la instalación, el cliente solicita la cancelación al puente Go y espera
el cierre del proceso antes de anunciar `Cancelado`. El puente detiene el árbol
del instalador real. Las líneas vacías NDJSON se rechazan, igual que en el
consumidor Go.

Los assets de `dist/assets` se copian al compilar desde `../assets`; no son una
segunda fuente de identidad visual.

## Validar

```sh
npm --prefix installer test
npm --prefix installer run typecheck
npm --prefix installer audit
python3 scripts/visual-smoke.py ./mem --installer installer/dist/cli.js
```

La última comprobación usa un HOME temporal: verifica la CLI en una PTY y
ejecuta instalación y reinstalación reales sin modificar la configuración de
usuario.
