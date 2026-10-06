# Temas de terminal

goMemory comparte la identidad de sus logos entre la consola, la TUI y los instaladores.

| Color | Valor |
| --- | --- |
| Abismo | `#061328` |
| Brillo | `#22e3f7` |
| Cian | `#06c1ee` |
| Azur | `#2c8ff0` |
| Índigo | `#3926e3` |
| Violeta | `#6b56d4` |

## Selección desde la TUI

Abre `mem tui`, pulsa `c` para Configuración y después `t` para cambiar entre **goMemory oscuro → goMemory claro → Matrix**. También puedes navegar hasta «Tema visual» y pulsar Enter. El cambio se aplica inmediatamente y se guarda en `.memory/settings.json` del proyecto; la elección guardada tiene prioridad sobre la variable de entorno en la TUI.

Matrix conserva la paleta verde original, incluidos los colores por tipo de memoria.

## Selección

```sh
GOMEMORY_THEME=dark mem tui
GOMEMORY_THEME=light mem tui
GOMEMORY_THEME=light mem help
GOMEMORY_THEME=matrix mem tui
```

Para guardar la elección en tu shell:

```sh
export GOMEMORY_THEME=dark
```

En PowerShell:

```powershell
$env:GOMEMORY_THEME = 'light'
mem tui
```

Con `auto`, o sin la variable, se consulta `COLORFGBG`: fondo 7 o 15 selecciona claro; cualquier otro valor selecciona oscuro. Si la terminal no proporciona esa pista, usa `dark` o `light` explícitamente.

El tema oscuro usa Abismo como fondo y Brillo/Cian para destacar acciones. El claro usa un fondo pálido y acentos Índigo/Violeta. Los colores auxiliares de texto y estado se ajustan al contraste de cada fondo.

La consola conserva el fondo de la terminal; la TUI dibuja el fondo del tema. Los banners y las animaciones respetan `NO_COLOR`, `TERM=dumb`, CI y la salida redirigida. El logo se adapta al ancho disponible y está embebido desde `assets/gomemory-terminal.txt`.

La paleta compartida vive en `assets/console-themes.json`. Define también el
gradiente del símbolo braille y los colores de aviso/error. El instalador
TypeScript copia estos assets al compilar; la TUI utiliza los mismos roles.

## Movimiento

```sh
mem --no-motion help
GOMEMORY_NO_MOTION=1 mem index
```

`GOMEMORY_REDUCED_MOTION=1` tiene el mismo efecto. Desactivar el movimiento
conserva el tema y los colores; `NO_COLOR` y `TERM=dumb` usan presentación plana.

Consulta [la evidencia TDD y el contrato visual](CLI-VISUAL-TDD.md) para reproducir
las comprobaciones de terminal, y [el instalador TypeScript](../installer/README.md)
para ejecutar la entrada opcional desde el repositorio.
