# Investigación — 038

Fecha: 2026-10-12. Investigación de diseño, no implementación ni nueva reproducción del fallo upstream.

## 1. Prioridad condicionada por capacidad y suficiencia

**Decision**: una directiva canónica pura en `domain/code_retrieval_policy.go`, compartida por `buildIntegrationBlock` y `memoryProtocolReminder`.

**Rationale**: `cmd_install.go:693–702` exige siempre el grafo externo y después recomienda la alternativa propia; `cmd_hook.go:1617–1621` solo repite la obligación externa. El mismo texto se entrega en instrucciones globales y MCP initialize (`newMCPServer`). Una regla condicional evita duplicación y no necesita inventar detección runtime/versionado.

**Alternatives considered**: excepción solo en AGENTS (recordatorio sigue contradictorio); un router nuevo (innecesario para un problema de instrucciones); forzar grafo propio para todo lenguaje (pierde cobertura).

## 2. Qué alternativas existen realmente

**Decision**: usar `search_code` para búsquedas Go cubiertas y comprimibles; `get_symbol` para firma/ubicación/relaciones, sin prometer compresión o cuerpos. Para cuerpos o proyectos/lenguajes no cubiertos, permitir fuente externa y lectura focalizada explícita.

**Rationale**: `registerCodeTools` declara grafo Go. `search_code` llama `compressDeliveredContext`; `get_symbol` devuelve metadatos y vecinos sin esa llamada. `formatCodeNodeDefinition` no lee cuerpos. `compressDeliveredContext` solo aplica compresión en nivel max, con compresor disponible y refs recuperables; sin omisiones retorna el original.

`pack_build` recupera memorias y artefactos Spec Kit con señales de grafo; no sustituye una consulta de cuerpos. `pack_compress` transforma texto ya disponible: si ese texto ya llegó al agente, no elimina su coste previo. No se aceptará el tamaño menor de una firma frente a un cuerpo como ahorro equivalente.

**Alternatives considered**: agregar cuerpos a get_symbol o un puente multilenguaje (amplía alcance y contrato); recomendar pack_compress después de cada snippet (duplicación y posible aumento del contexto).

## 3. Compatibilidad Codex y otros agentes

**Decision**: conservar rewrite Codex deshabilitado y los perfiles compatibles sin imponerles la excepción. Capacidades desconocidas no son soporte confirmado.

**Rationale**: `rewriteToolOutput` retorna nil para Codex; `CodexToolOutputHook` se conserva únicamente para retirarlo. `toolOutputIsExcluded` excluye, además, lecturas exactas y ciertos snippets: soporte general del host no garantiza compresión de cualquier herramienta.

La incompatibilidad upstream de Codex 0.162.1 es antecedente aportado (memoria 450), no una comprobación nueva de este comando. La documentación de Context7 `/openai/codex` consultada el 2026-10-12 muestra TUI y trazas locales sensibles, pero corresponde a fuente actual: no prueba qué interfaces están disponibles en la versión instalada. Durante ejecución se registrará la versión y se comprobará una interfaz de captura y un oráculo independiente de entrada del modelo; sin ambos y sin certificado verificable, SC-003 queda bloqueado.

**Alternatives considered**: reinstalar PostToolUse incompatible; asumir soporte de hooks con `codex exec`; usar eventos nuevos documentados en main como prueba de soporte en 0.162.1. Todas descartadas.

## 4. Migración efectiva de instrucciones

**Decision**: marcador de protocolo v8→v9; mantener delimitador final, baseline universal y contenido ajeno.

**Rationale**: `composeAgentFile` actualiza protocolo solo cuando falta el marcador vigente. Cambiar texto sin subir marcador no actualiza bloques v8 instalados. `protocolEnd` reconoce delimitador o límites legados. La instalación global escribe `~/.codex/AGENTS.md`; instalación por proyecto no genera AGENTS desde feature 021.

**Alternatives considered**: editar archivos personales manualmente (no distribuible); cambiar baseline universal (no contiene esta regla); reintroducir AGENTS por proyecto (regresión); reemplazar archivos completos (destruye trabajo ajeno).

## 5. Medición sobre respuestas equivalentes

**Decision**: bytes UTF-8 de `visible_text` capturado exactamente en la entrada del modelo como unidad primaria, mediante `codex-tui-visible-text-utf8-v1` en observación y referencia. La referencia se reproduce fuera de aceptación con el mismo punto de captura, versión de Codex y binario MCP; pares sin captura textual comprobable quedan `blocked`. Misma fuente/revisión/consulta/parámetros en ambos lados. Encabezados, metadatos y avisos incluidos; sumar recuperaciones necesarias para resolver cada consulta. Informe no equivale a tokens facturados.

**Rationale**: responde a aclaración A sin nueva dependencia de tokenizador. La captura del punto realmente visible evita que dos serializaciones del resultado MCP produzcan cifras distintas. Comparar misma recuperación sin compresión contra su salida real permite atribuir ahorro, sin confundir truncamiento, deduplicación, referencia histórica o eliminación de evidencia con compresión. Si no se puede capturar fielmente la respuesta completa visible, el par queda bloqueado.

**Alternatives considered**: tamaño del cuerpo únicamente (rechazado por usuario); comparar snippet externo con lista propia (contenido distinto); usar savings acumulado del proyecto (no atribuible a sesión); aplicar compresión a una salida ya vista y contar solo la segunda (oculta coste previo).

## 6. Investigación y fuentes

- pytest 9.1.1 consta como versión estable en [PyPI](https://pypi.org/project/pytest/9.1.1/) y declara Python ≥3.10, compatible con la CI 3.13; [pip-audit 2.10.1](https://pypi.org/project/pip-audit/2.10.1/) declara Python ≥3.10. Se fijan como candidatos del grupo de pruebas 038: T001 debe comprobarlos con Context7 y fuentes oficiales, generar `uv.lock` mediante `uv lock`, exportar transitivas con hashes y ejecutar `pip-audit --require-hashes` en cambio separado. La [documentación de uv](https://docs.astral.sh/uv/concepts/projects/sync/) describe bloqueo/sincronización y exportación; [pip-audit](https://github.com/pypa/pip-audit/blob/main/README.md) documenta auditoría de requisitos con hashes. T030 añade estas puertas a CI.
- CodeGraph sobre el checkout actual: funciones y relaciones citadas; detalle de `registerCodeTools` leído directamente porque la salida comprimida omitió el cuerpo y su ref volvió a resultar comprimida.
- R01: investigación independiente de superficies de instrucciones autorizada por Octopus y delegada en modo solo lectura. Confirmó migración y tests existentes.
- `codebase-memory-mcp.list_projects`: no lista el checkout actual; índice antiguo `go_memory` descartado como evidencia de estado actual.
- `go.mod`: versiones existentes; no se decide stack nuevo.
- `mem constitution` y copia Spec Kit v2.0.0: reglas de seguridad, capas, pruebas e idioma.
- Context7 `/openai/codex`, fuentes `docs/install.md` y `codex-rs/rollout-trace/README.md`: distingue TUI de exec y advierte sensibilidad de trazas locales; se verifica compatibilidad de captura al ejecutar.

Todas las incógnitas de diseño quedan resueltas. No hay pruebas de implementación ejecutadas ni porcentaje de ahorro demostrado en esta fase.
