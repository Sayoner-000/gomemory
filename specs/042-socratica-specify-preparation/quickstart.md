# Validación del recorrido Socrática → Spec Kit

Esta guía se ejecuta después de implementar la feature 042. En esta planificación no se ejecutaron sus casos ni se certificaron capacidades nuevas de los agentes. Los contratos de entrada y salida están en [socratica-flow.md](contracts/socratica-flow.md) y [agent-skill.md](contracts/agent-skill.md).

## Prerrequisitos y aislamiento

- Usar un checkout con la implementación de 042, Go según `go.mod` y los clientes que se vayan a comprobar. Las ejecuciones reales reutilizan la autenticación instalada de cada cliente y requieren su disponibilidad operativa.
- Usar un proyecto de aceptación desechable, inicializado con gomemory y Spec Kit, separado del proyecto donde se desarrolla la feature. No se genera la especificación de prueba dentro de la feature 042 ni se cambia su puntero activo.
- Confirmar con el catálogo activo de cada agente que puede cargar `socratica` y `speckit-specify`. Comprobar la instalación real del alias donde corresponde. No inferirlo de `command -v`.
- Utilizar datos sintéticos del [ejemplo de preparación](contracts/example-preparation.json). No incluir credenciales, información de personas reales o texto protegido en los reportes de aceptación.
- Para las pruebas automáticas de instalación, usar el aislamiento de HOME y los helpers ya existentes en las suites. No instalar o desinstalar globalmente sobre la configuración personal como parte del test.

## Puerta automática

Desde la raíz del repositorio implementado:

```sh
go build ./...
go vet ./...
go test ./... -count=1
go test -race ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli -count=1
bash scripts/coverage.sh
```

Son órdenes existentes del proyecto; respetar sus cachés, permisos y entorno de CI. El informe de cobertura debe mantener el mínimo constitucional de 80 % y la línea base aplicable. No rebajar puertas ante un fallo.

Antes de afirmar cobertura de 042, comprobar que existen y se ejecutaron casos nuevos para: validación por etapas, límite acumulativo de preguntas, revisión obsoleta, identidad del render, replay con concesión única, migración desde v2, rollback, payload privado, reserva entre procesos, sanitización, consulta local sin ejecución y wrapper/paridad de skills. Una suite verde sin esos casos no demuestra la feature.

Construcción del binario de aceptación, si hace falta:

```sh
go build -o /private/tmp/mem-042 ./infrastructure/
```

En Windows elegir una ruta temporal propia y extensión `.exe`; no depender de `TMPDIR`. No sustituir el binario personal antes de verificar el binario construido.

El diagnóstico real existente puede ejecutarse de forma opt-in:

```sh
GOMEMORY_TEST_REAL_HARNESS=claude go test ./adapters/secondary/generator -run '^TestRealSocraticaHarness$' -v -count=1
```

Se puede repetir con `codex` u `opencode`. Ese diagnóstico solo cubre el generador de refinamiento existente; no sustituye los recorridos siguientes ni demuestra carga de skill o traspaso.

## Caso A: preparación sin generar una especificación

1. En un agente conectado al proyecto desechable, iniciar `socratica_flow` con `action=start`, el cliente real, `mode=specify` y la idea del ejemplo.
2. Copiar sucesivamente las secciones `intent`, `scope` y `stories` del ejemplo en `prepare(operation=set)`, usando el `flow_id` y la revisión devueltos. Después de cada sección, usar `prepare(operation=validate)` para su etapa antes de avanzar. No rellenar a mano marcas de validez.
3. Comprobar `validation`. Debe devolver la entrada completa para revisión, sin generar archivos de especificación ni ejecutar la exportación descrita.
4. Comparar el texto con el objetivo, alcance, supuestos, historias y criterios del ejemplo. Deben conservarse sus identidades y relaciones. La persona decide si representa su necesidad.

Resultado esperado: revisión completa y recuperable, hasta tres preguntas acumuladas y cero especificaciones antes de aprobar.

## Caso B: bloqueos, edición y preguntas

1. Quitar un criterio o introducir una referencia a una historia inexistente en el borrador. Comprobar la etapa: muestra el campo y la relación afectados, sin habilitar revisión/aprobación.
2. Añadir un pendiente abierto bloqueante y omitir una respuesta sobre él. Sigue bloqueando la revisión. Resolverlo exige actualizar explícitamente los datos y justificar la resolución.
3. Registrar tres preguntas a lo largo del flujo. Retomar y editar; una cuarta se rechaza. Se sigue permitiendo completar el borrador mediante aportes voluntarios.
4. Con una revisión válida, cambiar `scope`. Una aprobación con la revisión anterior se rechaza. Corregir, comprobar y revisar la versión nueva antes de aprobar.
5. Intentar `rewrite`, edición textual libre, `send_original` y ejecución genérica local para este modo. Ninguna puede eludir la preparación.

Resultado esperado: se conservan original y decisiones; las condiciones faltantes impiden el avance y ninguna omisión se convierte en respuesta supuesta.

## Caso C: disponibilidad, aprobación y replay

1. En el proyecto desechable, usar un entorno de agente sin capacidad de cargar `speckit-specify`. Llegar a revisión/aprobación. Debe comunicar la dependencia y conservar la entrada; no llamar a `ack` ni consumir autorización.
2. Volver a un entorno con esa capacidad. Revisar la versión vigente, elegir «Aprobar y generar especificación» y obtener el despacho. Confirmar con un nonce nuevo.
3. Solo la primera confirmación correcta devuelve `execution_granted=true`. Entregar exactamente `approved_prompt` a la skill. Repetir aprobación y confirmación no inicia otra generación.
4. En una prueba controlada, perder la primera respuesta de confirmación y repetir el nonce. Si la transición se confirmó, se obtiene acuse sin nueva concesión; se explica el consumo y se exige reconciliación explícita. No ejecutar automáticamente desde el replay.
5. Probar dos confirmaciones concurrentes en la suite determinista: una sola concesión tras commit; con rollback, ninguna.

Resultado esperado: ausencia del generador no consume autorización y cada autorización concede como máximo una ejecución. Un acuse o estado `dispatched` no demuestra que se produjo una especificación.

## Caso D: resultado real en los tres agentes

Repetir un recorrido completo en un proyecto de aceptación por agente:

| Agente | Invocación que debe comprobarse |
| --- | --- |
| Claude Code | `/socratica-hu Quiero permitir que una persona exporte su historial autorizado` |
| Codex | `$socratica prepara para speckit-specify: Quiero permitir que una persona exporte su historial autorizado` |
| OpenCode | `/socratica-hu Quiero permitir que una persona exporte su historial autorizado` |

1. Observar el recorrido guiado y elegir la aprobación explícita del texto final.
2. Verificar la concesión y la invocación de Spec Kit. Abrir el `spec.md` y su checklist reales, no únicamente un reporte del agente.
3. Comprobar que el alcance no añade exportación de terceros ni cambios de permisos; rastrear C1 y C2 hasta US1 y sus condiciones en la especificación, conservando IDs o una tabla de correspondencia.
4. El agente comunica resultado y rutas con `report`. Verificar que coinciden con archivos existentes en ese proyecto. Si falla la generación, comunicar `failed/uncertain` sin afirmar éxito ni reintentar con la autorización consumida.
5. Registrar evidencia con cliente y versión, invocación, revisión, despacho, rutas, trazabilidad y resultado. No atribuir éxito a un agente cuya demostración no se ejecutó.

También comprobar en cada agente una entrada equivalente en lenguaje natural. La presencia del wrapper en disco se comprueba además en las pruebas de instalación.

## Caso E: recuperación, privacidad y borrado

1. En modo persistente, interrumpir una preparación con historias y un pendiente. Retomarla y comparar decisiones, relaciones, revisión y preguntas consumidas. Consultar desde CLI/TUI no debe lanzar generación ni consumir autorización.
2. En modo privado, preparar y revisar usando texto sintético identificable. Inspeccionar almacenamiento durable, historial, temporales y locks: no contienen original, preguntas, preparación, revisión ni rutas privadas. Solo puede existir la reserva de metadatos sin texto.
3. Abrir otra instancia MCP para el mismo proyecto/cliente. Debe ver la reserva sin payload, no crear otro pendiente ni reconstruir una autorización. Perder la instancia original produce `private_state_unavailable`; aprobación/confirmación/reenvío se rechazan y la cancelación explícita permite empezar otro flujo.
4. Cambiar el ajuste privado después de iniciar el flujo no vuelca su payload a disco. Un fallo de commit no publica una versión nueva en memoria.
5. Incluir un fragmento sintético protegido y comprobar que se retira antes de renderizar, persistir o entregar. Si la protección deja un dato esencial sin resolver, la preparación se bloquea.
6. Aplicar la sanitización y retención existentes sobre un flujo persistente: cubren el agregado y la revisión. Un cambio de contenido invalida la aprobación pendiente. El borrado no deja preparaciones o revisiones huérfanas.

## Caso F: instalación, compatibilidad y migración

En los entornos aislados de pruebas, instalar dos veces y comprobar cero escrituras en la segunda; verificar wrapper Claude/OpenCode y paridad de las dos copias de la skill. Desinstalar y comprobar que se retiran los artefactos gestionados y permanecen archivos ajenos. Repetir con un agente ausente: no se crea su raíz.

Abrir una base vacía y una base de catálogo v2 con flujos `refine/scrutiny`; comprobar la migración aditiva, conservación de datos y lectura de preparación nula. Simular fallo durante la migración y comprobar rollback. Verificar que el siguiente arranque reconoce el catálogo actualizado. Las suites previas de refinamiento/escrutinio y la invocación independiente de Spec Kit deben seguir funcionando.

## Cierre de aceptación

Los nueve resultados SC de [spec.md](spec.md) deben tener evidencia identificable. La puerta automática y los tres casos reales se registran por separado. Si falta alguno, el estado es aceptación pendiente, con la causa y el siguiente paso; no se declara la funcionalidad completa por haber terminado este diseño.
