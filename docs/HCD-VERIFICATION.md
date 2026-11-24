# Verificación HCD sin repeticiones acumuladas

Desde v2.55.3, las skills `hcd-implement` y `hcd-verify` separan la comprobación de la tarea actual de la revalidación final. El motor conserva sus reglas de vigencia y cierre.

## Durante la implementación

Ejecuta el test focalizado mediante `mem hcd verify`: primero el rojo, después el cambio y el verde. El comando ejecuta la prueba y registra su resultado; no hace falta correrla antes con otra herramienta ni repetirla por MCP. Atiende cualquier fallo real antes de seguir.

Cambiar código invalida la evidencia de tareas anteriores porque la huella abarca el repositorio. Mientras queden tareas pendientes, esas tareas caducadas cuentan como progreso: el motor sigue en `implement`, no las lista como no verificadas y `analyze` no las señala. Continúa con la tarea siguiente; no recorras la lista caducada después de cada edición. Hacer commit sin cambiar contenido no caduca nada (v2.59.0).

## Antes del cierre

Con el código estable y sin tareas pendientes, revalida las caducadas con `mem hcd verify --stale --json --client <agente>`, que reejecuta el comando propio de cada una. Un comando puede cubrir varias tareas con `--task 2,3` si realmente prueba ambas. El estado devuelto por `verify` evita un `show` inmediato cuando no hubo cambios, interrupciones ni trabajo concurrente. El cierre sigue exigiendo evidencia vigente y aprobación humana.

## Comparación reproducible

```bash
go test ./domain -run TestHCDSchedulingComparison -count=1 -v
```

El escenario incorpora diez tareas, cambiando la huella en cada paso. Usa el predicado real de vigencia del dominio y compara dos políticas:

| Política | Comprobaciones asociadas a tareas | Registros agrupados de evidencia |
|---|---:|---:|
| Revalidar todas las tareas tras cada cambio | 55 | 10 |
| Verificar la actual y revalidar las anteriores al final | 19 | 11 |

Son 36 comprobaciones menos (65,5 %) en este escenario. El test modela la planificación: no ejecuta los comandos de cada tarea ni mide duración, tokens o llamadas MCP de una sesión de Pi. Los registros agrupados incluso aumentan de diez a once. No debe interpretarse como un ahorro medido de transporte.

## Una huella por comando (v2.56.0)

La huella identifica el código evaluado: `HEAD` más el contenido de los archivos cambiados y sin seguimiento, sin contar `hcd/`. Calcularla lanza cuatro procesos git y lee cada archivo cambiado, así que su coste crece con el tamaño del cambio en curso.

Hasta v2.55.3, `clarify`, `approve`, `close`, `cancel`, `promote`, `rename` y `review` leían la unidad dos veces y calculaban la huella en cada lectura. Entre una lectura y otra solo escriben en `hcd/` o en la base de datos, y la segunda lectura reutiliza ahora la huella de la primera. `verify` la sigue calculando dos veces: el comando que ejecuta puede generar archivos o cambiar el código, y la evidencia tiene que reflejarlo.

```bash
go test ./application/usecases -run TestHCD061FingerprintPorComando -count=1 -v
```

El test cuenta las huellas con un workspace instrumentado. En el escenario, las nueve llamadas de esos siete comandos pasan de dos huellas a una y `verify` se mantiene en dos. Cuenta llamadas; no mide tiempo.

Desde esta versión, `--client pi` es un cliente válido de `mem hcd`. Antes el motor lo rechazaba y Pi registraba sus aprobaciones como `terminal`.

## Revalidación única de caducadas con `--stale` (v2.57.0)

Hasta v2.56.0, una tarea que ya había pasado y caducaba al editar otro archivo volvía al estado `unverified`, igual que una casilla marcada a mano sin evidencia. Al cerrar con varias tareas caducadas, los agentes terminaban revalidándolas una por una en secuencia.

Desde v2.57.0 el motor separa los dos casos:

- `stale`: la tarea tuvo una evidencia aprobada anterior, pero la huella del repositorio o la revisión de trabajo cambió después.
- `unverified`: la casilla está marcada como hecha sin ninguna evidencia aprobada previa.

El cierre sigue exigiendo evidencia vigente, pero ahora basta una llamada para revalidar todas las caducadas cuando una suite de regresión cubre los cambios:

```bash
mem hcd verify --stale --json --client pi -- 'go test ./...'
```

`--stale` resuelve las tareas en estado `stale`, ejecuta el comando una vez y registra una única evidencia vigente sobre la huella actual. Rechaza combinarse con `--task` o `--expect-fail`, y devuelve error de validación si no hay tareas caducadas.

## Huella por contenido y `--stale` sin comando (v2.59.0)

La huella ya no incluye `HEAD`. Identifica el contenido del árbol de trabajo: el id de blob de git de cada archivo no ignorado, sin contar `hcd/`. Los archivos sin cambios toman su id del índice y solo se hashean los modificados y los sin seguimiento, con `git hash-object --stdin-paths`, que aplica los mismos filtros que `git add`. Así el id de un archivo es el mismo antes y después de prepararlo o commitearlo, y un commit a mitad de la unidad deja la evidencia vigente. Borrar un archivo cambia la huella; commitear ese borrado, no. La huella pasa a tener la forma `tree+<12 hex>`; al actualizar, la evidencia de las unidades activas caduca una sola vez.

```bash
mem hcd verify --stale --json --client pi
```

Sin comando, `--stale` busca para cada tarea caducada el último comando con el que pasó y lo vuelve a ejecutar. Las tareas que comparten comando se agrupan en una evidencia. Una tarea que solo pasó con evidencia manual se queda caducada y se vuelve a observar con `--task N --manual`; si todas son manuales, el motor lo indica con un error de validación. Con un comando tras `--`, `--stale` se comporta como en v2.57.0.

```bash
go test ./adapters/secondary/hcdfiles -run TestFingerprint -count=1 -v
go test ./application/usecases -run TestHCDVerifyStale -count=1 -v
go test ./domain -run TestHCD064 -count=1 -v
```

`WorkRevision` no cambia: editar el texto del checklist o de la definición sigue caducando toda la evidencia.

## Actualización

Las instrucciones se distribuyen con las plantillas de gomemory. Actualiza el binario con `mem update` y comprueba que tu integración cargue las skills nuevas. Si un instalador externo mantiene su propia copia, también deberá actualizarla; cambiar el binario por sí solo no demuestra que Pi esté leyendo las nuevas instrucciones.
