# Contrato: script del hook `speckit.gomemory-context.update`

Ver research.md #3 y #4. Este documento fija el contrato de comportamiento
del script (`scripts/bash/update-gomemory-context.sh` /
`scripts/powershell/update-gomemory-context.ps1`) que ejecuta el hook
`before_specify` (y opcionalmente `before_plan`/`before_clarify`).
`/speckit-tasks` desglosa la implementación concreta a partir de este
contrato.

## Entrada

- Ninguna entrada obligatoria (a diferencia de
  `update-agent-context.sh [plan_path]`, este script no depende de un path
  de plan).
- Variables de entorno/contexto disponibles: directorio de trabajo actual
  (raíz del proyecto donde vive `.specify/`).

## Comportamiento (orden estricto)

1. **Localizar el binario `mem`**: probar `./mem` (raíz del proyecto,
   artefacto que deja `mem install`) y, si no existe, `mem` en `PATH`. Si
   ninguno se encuentra → ir a paso 4 (salida vacía, éxito).
2. **Leer el interruptor**: `./mem settings --show` (o lectura directa de
   `.memory/settings.json`, clave `speckit_context_disabled`). Si el valor
   es `true` → ir a paso 4 (salida vacía, éxito) — FR-009.
3. **Obtener el resumen**: ejecutar `./mem context` y capturar stdout.
   - Si el comando falla (exit code ≠ 0) o no hay proyecto con memoria
     inicializada → ir a paso 4 (salida vacía, éxito) — FR-004.
   - Si el comando tiene éxito pero el contenido es solo el encabezado sin
     secciones (proyecto nuevo sin memorias) → igualmente se puede emitir
     tal cual (es información válida: "sin historial aún"), no es un error.
4. **Salida**: el script SIEMPRE termina con código de salida `0`
   (nunca hace fallar el hook mandatorio) — imprime el resumen obtenido en
   paso 3, o nada si se llegó aquí desde el paso 1/2/3.

## Postcondiciones (invariantes verificables)

- El script NUNCA bloquea el flujo de `/speckit-specify`: código de salida
  siempre `0`.
- El script NUNCA escribe en el grafo de código externo ni en
  `.memory/settings.json` — es de solo lectura (FR-008).
- El script NUNCA escanea `specs/` — obtiene el resumen exclusivamente vía
  `mem context` (FR-002).
- Tiempo de ejecución esperado: milisegundos a bajo segundo — una lectura
  de SQLite local ya cacheada/acotada por `Budget`, sin llamadas de red
  obligatorias (SC-002).

## Escenarios de prueba (mapean a Acceptance Scenarios de spec.md)

| Escenario | Entrada | Salida esperada |
|-----------|---------|------------------|
| Proyecto con historial, interruptor activado | `mem` presente, settings ok, hay memorias | stdout = resumen Markdown de `mem context` |
| Proyecto nuevo sin memorias | `mem` presente, sin memorias guardadas | stdout = encabezado sin secciones (o vacío), exit 0 |
| Interruptor apagado | `speckit_context_disabled=true` | stdout vacío, exit 0, sin llamar `mem context` |
| `mem` no encontrado (ni `./mem` ni PATH) | binario ausente | stdout vacío, exit 0 |
| `mem context` falla | proyecto sin inicializar / error interno | stdout vacío, exit 0 |
