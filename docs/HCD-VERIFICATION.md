# Verificación HCD sin repeticiones acumuladas

Desde v2.55.3, las skills `hcd-implement` y `hcd-verify` separan la comprobación de la tarea actual de la revalidación final. El motor conserva sus reglas de vigencia y cierre.

## Durante la implementación

Ejecuta el test focalizado mediante `mem hcd verify`: primero el rojo, después el cambio y el verde. El comando ejecuta la prueba y registra su resultado; no hace falta correrla antes con otra herramienta ni repetirla por MCP. Atiende cualquier fallo real antes de seguir.

Cambiar código puede invalidar la evidencia de tareas anteriores porque la huella abarca el repositorio. Conserva esos resultados como historial y continúa con la tarea siguiente; no recorras toda la lista caducada después de cada edición.

## Antes del cierre

Con el código estable, vuelve a comprobar las tareas sin evidencia vigente. Un comando puede cubrir varias tareas con `--task 2,3` si realmente prueba ambas. El estado devuelto por `verify` evita un `show` inmediato cuando no hubo cambios, interrupciones ni trabajo concurrente. El cierre sigue exigiendo evidencia vigente y aprobación humana.

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

## Actualización

Las instrucciones se distribuyen con las plantillas de gomemory. Actualiza el binario con `mem update` y comprueba que tu integración cargue las skills nuevas. Si un instalador externo mantiene su propia copia, también deberá actualizarla; cambiar el binario por sí solo no demuestra que Pi esté leyendo las nuevas instrucciones.
