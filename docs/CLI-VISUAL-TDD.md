# Presentación de consola y evidencia TDD

## Contrato visual

- `assets/gomemory-terminal.txt` conserva el símbolo braille de la gota y el aro,
  separado del nombre y de los textos del encabezado.
- `assets/console-themes.json` define las paletas dark, light y Matrix, el
  gradiente del símbolo y los colores de aviso/error. Go y TypeScript consumen
  el mismo archivo.
- `console.Layout` presenta encabezados, secciones, tablas y mensajes humanos.
  Mide columnas Unicode/ANSI; una tabla que no cabe pasa a fichas verticales.
- Los banners completos se reservan para ayuda y bienvenida/instalación. Las
  consultas frecuentes usan encabezados compactos.
- JSON, MCP, hooks, contexto y documentos exportados conservan sus canales de
  datos. La decoración no se aplica a esas salidas.
- Las ayudas solicitadas van a stdout; los errores de argumentos y su ayuda
  permanecen en stderr. Los parsers respetan `SetOutput`.

## Inventario de presentación

| Familia | Presentación |
| --- | --- |
| Ayuda global | Inicio rápido, catálogo por categorías, ejemplos y opciones |
| Ayudas registradas | Sintaxis y opciones generadas por el parser del comando |
| Familias con parser manual | `mem help <comando>` y `<comando> --help` sin ejecutar acciones |
| Listado, búsqueda y sesiones | Tablas adaptables; resumen y estados vacíos |
| Uso de contexto | Resumen, tablas por operación/canal y unidades explícitas |
| Configuración, proyecto y diagnóstico | Secciones, valores, estados y remedios |
| Instalación | Marca compartida, estados y resumen; eventos opt-in para TypeScript |
| TUI | Encabezado compacto compartido y acciones en estados vacíos |
| Datos y protocolos | Sin banner ni transformación visual de los documentos |

## Ciclos rojo → verde observados

Fecha de ejecución: **2026-10-06**. Las pruebas se añadieron antes de implementar
los comportamientos siguientes; las salidas rojas se observaron durante la sesión.

| Bloque | Evidencia roja | Cierre |
| --- | --- | --- |
| Layout | `NewLayout` no existía | Ancho Unicode, temas, tabla vertical y salida automática intacta |
| Marca | Banners de consulta excesivos, desbordamiento a 20 columnas y spinner activo con movimiento deshabilitado | Variantes adaptables y controles de movimiento independientes del color |
| Help | Faltaban inicio rápido, categoría de código y ayuda enfocada | Catálogo y ayuda por familia |
| Ayudas registradas | Los parsers imprimían formatos distintos | Formato compartido a partir de flags registrados |
| Uso | No existía el renderizador humano de tablas | Unidades y valores conservados a 40/80/120 columnas |
| Eventos | Faltaban flag y protocolo de eventos | NDJSON v1, logs separados, pasos y cierre reales |
| Agentes vacíos | `none` rechazado; selección vacía reemplazada por la guardada | Selección explícita preservada en Go y TypeScript |
| Instalador TS | Módulo de implementación ausente | Parseo, checksum, plataformas y consumo de NDJSON |
| Veracidad del cierre TS | Un `complete: ok` ocultaba un paso con aviso; `fail` con código 0 se aceptaba | Ambos casos rechazados |
| TUI | Sin siguiente acción y desbordamiento del estado vacío a 40 columnas | Ayudas accionables y presupuesto de ancho/alto respetado |
| Instaladores portables | Usaban un símbolo distinto | Asset sincronizado con Bash y PowerShell |
| Canales de errores | Ayuda de un flag inválido invadía stdout con `--json` | Error y ayuda en stderr |
| Familias manuales | `session --help` intentaba ejecutar un subcomando inválido | Ayuda segura sin dependencias |
| Comprobación PTY | Espacios finales desbordaban la ayuda a 40 columnas | Wrap seguido de ajuste estricto por columnas |
| Estados por tema | El resumen usaba colores ANSI genéricos | Avisos y errores con contraste adecuado por tema |
| Logo TS | Faltaban azul y violeta en el gradiente | Gradiente compartido con Go y símbolo braille restaurado |
| Instalación nativa conectada | No existía `NewFlow`; el binario anterior no mostraba el recorrido `┌ │ └` | Preguntas conectadas y resultados reales desde el mismo binario Go |
| CI con PTY | Se activaban preguntas en CI si existía una pseudo-TTY | CI fuerza modo no interactivo |
| C-001 | La consulta literal `json` y textos `--json` en la búsqueda suprimían la cabecera | La cabecera no depende del contenido de la consulta |
| C-002 | `LIGHT` resolvía dark en TS y light en Go | Normalización y 16 casos compartidos de resolución |
| Cancelación C-001 | Tras terminar el padre, el escritor de `seed` avanzaba de 1 a 25 | El puente detiene el árbol y los clientes esperan su cierre |
| NDJSON C-002 | TS ignoraba líneas vacías que Go rechazaba | Ambos fallan cerrado ante líneas vacías y blancas |
| Ayuda C-003 | `mem install --events --help` terminaba 2 | Ayuda prioritaria antes de validar o instalar |
| Mensajes globales S-001 | Una ruta global larga desbordaba 80 columnas | Mensajes globales pasan por el layout humano |

La captura de referencias y la definición del inventario son tareas de inspección;
no se les atribuye un ciclo de pruebas de comportamiento.

## Verificaciones reproducibles

Desde la raíz del repositorio:

```sh
go test ./...
npm --prefix installer test
npm --prefix installer run typecheck
npm --prefix installer audit
bash -n scripts/install.sh
go build -o ./mem ./infrastructure
python3 scripts/visual-smoke.py ./mem --installer installer/dist/cli.js
```

La suite Go completa, incluidos contratos e integración, pasó. Los **17 tests
del instalador**, el chequeo de tipos y `npm audit` pasaron; la auditoría no
reportó vulnerabilidades.

`visual-smoke.py` comprueba help, ayuda de instalación, list, search, usage,
settings y doctor a **40, 80 y 120 columnas**, en los tres temas. También
decodifica `usage --json` en PTY, valida eventos de instalación, un destino
inválido y una reinstalación desde TypeScript. Todo ocurre en un HOME aislado.

También ejecuta `mem install` en una PTY cuyo PATH no contiene Node.js ni npm.
Comprueba el recorrido nativo, las respuestas con teclado y la cancelación con
Esc sin escribir en el proyecto destino.

La cancelación tiene una regresión independiente:

```sh
python3 scripts/install-cancel-smoke.py ./mem --installer installer/dist/cli.js
```

Esta prueba bloquea una instalación real en un `seed` controlado que lanza un
escritor descendiente. Envía SIGINT/SIGTERM solo al padre y comprueba que el
contador no cambia después del cierre. Incluye cliente nativo, modo `--events`,
cliente TS y un error de protocolo. Las pruebas operativas se ejecutaron en
macOS; la variante Windows se comprobó mediante compilación cruzada.

## Experiencia incluida en el binario

En una terminal interactiva compatible, `mem install .` presenta preguntas
conectadas y transmite los resultados reales de instalación a un recorrido
visual en Go. No inicia npm, Node.js ni el instalador TypeScript. Con `--yes`,
CI o entrada/salida redirigida se conserva el flujo automático. Las terminales
sin color o estrechas conservan las preguntas en texto plano.

El instalador TypeScript permanece como entrada opcional para quien lo ejecute
explícitamente; la experiencia principal de `mem` no depende de él.

La TUI tiene pruebas de renderizado de ancho/alto y temas. PowerShell tiene una
comprobación de paridad del asset; su ejecución nativa requiere Windows y no se
verificó en esta sesión macOS.

## Contrato de instalación v1

```sh
mem install /ruta/al/proyecto --events --agents none
```

stdout contiene NDJSON; stderr conserva los mensajes de diagnóstico. Cada evento
incluye `contract_version: 1` y un `type`:

- `start`: comienzo de la ejecución nativa.
- `step`: resultado `ok`, `warn` o `fail`, con nombre y detalle/remedio opcionales.
- `complete`: estado final y `exit_code` del proceso.

El cliente TypeScript exige un cierre, rechaza eventos truncados o de otra
versión y verifica coherencia entre los avisos, el estado y el código de salida.
El modo normal de `mem install` conserva sus mensajes y comportamiento nativo.

Las líneas vacías o compuestas solo por espacios no pertenecen al contrato.
Cancelar detiene las escrituras futuras antes de anunciar el cierre; los pasos
ya completados no se revierten automáticamente.
