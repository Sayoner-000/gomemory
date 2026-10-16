# Contrato TUI: pantalla Socratica

Se integra en `adapters/primary/tui` como `screenSocratica` y `screenSocraticaDetail`, con una entrada nueva en el menú principal. Los estilos van en `styles.go` como tokens con nombre (sin literales de color en el render).

## Tokens visuales (FR-035)

| Token | Valor |
|---|---|
| `socDone` | `#3DD68C` |
| `socCurrent` | `#F2C46D` |
| `socUpcoming` | atenuado del tema vigente |
| `socFg` | `#E6EDF3` |
| `socLabel` | `#7D8590` |
| `socControl` | `#262B33` |
| `socWarn` | `#E3B341` |

Sin animaciones, sin emojis. El título va en mayúsculas («SOCRATICA») con subrayado dorado en el segmento inicial y una línea horizontal tenue de continuación.

## Pantallas

- **Acuse inmediato**: al pedir un refinamiento, la pantalla muestra «Generando preguntas con <agente>…» antes de despachar el `tea.Cmd` del generador (FR-017).
- **Lista**: el seguimiento «Tu prompt | Preguntas | Reescritura | Revisión» del flujo pendiente (si lo hay), las filas del historial (fecha · agente · modo · N→M palabras · primera línea del original con elipsis) y el estado de intercepción por agente. Teclas: `enter` abre el detalle, `i` alterna la intercepción del agente seleccionado (respeta FR-019), `r` abre el refinamiento (generador R3), `p` alterna el modo privado y `esc` vuelve.
- **Detalle/Revisión**: las acciones arriba (Ejecutar · Editar · Copiar · Cancelar) y debajo las tarjetas «Original» y «Propuesta» (esta con borde dorado), con desplazamiento vertical independiente. Al final, la línea de resumen. En modo `scrutiny`, la tarjeta «Propuesta» se sustituye por la síntesis: reformulación, supuestos (hecho/hipótesis), mejor argumento a favor, objeción más fuerte, alternativas, evidencia faltante y criterios.
- **Fallo del generador**: «Reintentar · Enviar mi original · Cancelar», con el original intacto (FR-016).
- **Preguntas**: un marco de trazo grueso por pregunta, de hasta 3 filas, con su número. Opciones como chips, `o` Otra, `s` Omitir, `S` Omitir el resto, contador «k/n respondidas» y ✓ verde al responder (el ✓ es un glifo de texto, no un emoji).

## Anchos

- ≥ 100 columnas: tarjetas lado a lado.
- 60–99: tarjetas apiladas.
- < 60: aviso de una línea «Ventana demasiado estrecha: original conservado» y el original visible con desplazamiento horizontal. Las filas se acotan con elipsis y nunca se ajustan por accidente.

## Pureza

`View()` solo lee el modelo. Todo cambio de estado, persistencia, generación y despacho pasa por `SocraticaFlow.Apply/Resume/Execute`, invocados dentro de `tea.Cmd`; `Update` solo guarda el flujo devuelto. La TUI no importa `domain.Reduce` (constitución §3/§5). Una prueba verifica que llamar a `View()` repetidamente no cambia el repositorio falso ni el modelo.
