# Modelo lógico — 038

Sin tablas, migraciones o estado global nuevo. La política es texto puro; las entidades de evidencia pertenecen al informe local de validación.

## CapabilityProfile

- `agent`, `version`: identifican la versión realmente evaluada; par único en un informe.
- `rewrite_status`: `supported`, `unsupported` o `unknown`, con `evidence` y fecha.
- `retrieval_coverage`: lenguaje/proyecto/consulta cubiertos por cada vía disponible.
- `exact_read_exclusions`: rutas para las que no debe inferirse compresión automática.

No convertir `unknown` en `supported`; proveedor conectado no equivale a proyecto indexado. Capacidad y suficiencia son condiciones diferentes.

## RetrievalPolicy

- `policy_version`: v9 del protocolo; una sola directiva canónica.
- `preferred_condition`: sin rewrite compatible, fuente comprimida suficiente disponible.
- `fallback_condition`: indisponibilidad, cobertura incompleta o evidencia insuficiente.
- `limitations`: Go/estructural en grafo propio; sin promesa de cuerpos o ahorro por nombre.

Relación: múltiples superficies contienen la misma directiva. No hay mutación de perfil en runtime ni nueva selección de adaptadores.

## QueryCase

- `id`: único dentro de `queries`; `prompt`: solicitud natural que no nombra herramientas.
- `required_facts`, `required_references`: listas previas de objetos `{id, text}` con IDs únicos por consulta.
- `comparison_key`: `{tool_family, source_scope, source_revision, query, filters, limit}`; `filters` conserva el orden y `limit` es entero positivo.
- `eligibility`: `eligible` o `ineligible` con `eligibility_reason`, definido antes de aceptación.
- `manual_tool_coaching`: booleano; `true` impide cierre autónomo.

Requiere al menos tres consultas elegibles. Una consulta que necesita un cuerpo no es equivalente a una respuesta con firma sola.

## RecoveryObservation

- `id`, `query_id`, `side`: ID de llamada único global, consulta existente y lado `observed`/`reference`.
- `tool`, `arguments`, `route`, `compression`, `comparison_key`: llamada real y clave de equivalencia; la referencia usa `compression=none`.
- `visible_text`: texto completo capturado en la entrada del modelo, incluidos encabezados, metadatos y avisos; no se recompone desde el resultado MCP crudo.
- `capture`: punto, sesión, ruta y SHA-256 de traza, versión del agente, SHA-256 del binario MCP y codec. El punto `unsupported` bloquea medición.

`visible_bytes` se deriva exclusivamente de `len(visible_text.encode('utf-8'))` con `codex-tui-visible-text-utf8-v1`; no es un campo de entrada. Lecturas adicionales aparecen como otras llamadas observadas y se atribuyen una sola vez.

## CaptureCertification

`surface_checks.capture_verified.evidence_ref` apunta al archivo local `.memory/validation/038/capture-certification.json`, validado con [capture-certification.schema.json](contracts/capture-certification.schema.json). Registra versión Codex y hash MCP, fuente del adaptador y oráculo independiente de entrada del modelo, hash de sus extractores, dos probes (`observed` y `reference`) con sesión/llamada/marker, fixture, trazas y hashes, y revisión de procedencia responsable. El evaluador comprueba estructura, rutas, archivos, hashes, igualdad byte a byte, lados y correlación con la sesión; `passed=true` sin certificado válido no permite `closed`. La revisión de implementación del oráculo confirma que el punto observado corresponde a la entrada real del modelo. Si la versión no ofrece ese punto, el caso queda `blocked`.

Datos sintéticos o públicos en el banco de prueba; no guardar credenciales ni transcripciones privadas completas en el repositorio.

## ComparisonPair

- `query_id`, `paired_calls`, `additional_observed_call_ids`: vínculos explícitos entre llamadas equivalentes y recuperaciones adicionales solo observadas.
- `review`: responsable, equivalencia comprobada, conservación y referencia a llamada observada para cada hecho/referencia exigidos.
- El informe deriva `reference_complete_bytes` y `observed_complete_bytes` como enteros no negativos; la referencia debe ser mayor que cero. No son campos de entrada.
- El informe fija `unit=utf8_bytes` y `measurement_method=codex-tui-visible-text-utf8-v1` para ambos lados.
- `delta_bytes = reference_complete_bytes - observed_complete_bytes`.
- `saving_percent = 100 * delta_bytes / reference_complete_bytes`.

No mezclar unidades, versiones o parámetros. Contar respuestas adicionales en el lado correspondiente. Deltas negativos se conservan; no se recortan a cero. No sumar coste de descubrimiento solo en un lado para fabricar ahorro.

## ValidationReport

- `report_version`: `1`; `status`: `pending`, `blocked` o `closed`.
- `agent_profile`, `session_id`, `manifest_digest`, `repository_revision`.
- `comparisons`: al menos tres pares válidos para cerrar.
- `aggregate_reference_bytes`, `aggregate_observed_bytes`, `delta_bytes`, `saving_percent`.
- `surface_checks`, `regression_checks` (incluido `pytest_ci`), `failure_reasons`.
- `cause`: incompatibilidad, conflicto de instrucciones, ausencia de uso, fallo de recuperación o medición insuficiente; puede haber más de una con evidencia separada.

La entrada del evaluador es distinta del informe: tiene los ocho campos raíz y relaciones fijados en [contracts/validation-evidence.schema.json](contracts/validation-evidence.schema.json). La muestra [validation-evidence.example.json](contracts/validation-evidence.example.json) prueba la forma de entrada, pero por ser sintética no acredita aceptación.

### Transiciones

`pending → blocked`: falta TUI, captura completa o referencia comparable.

`pending/blocked → closed`: nueva evidencia real satisface todos los requisitos del contrato, tres rutas previstas y conservación de hechos, ahorro agregado positivo y regresiones satisfechas.

`closed → pending`: nueva revisión invalida instrucciones, equivalencia o evidencia; conservar informe anterior y generar uno nuevo, no sobrescribir su procedencia.

Un test simulado o un reporte de savings acumulado nunca permite la transición a `closed`.

## FallbackObservation

`fallback-manifest.json` fija antes de la TUI una solicitud natural del cuerpo literal de una función Go conocida, su símbolo/archivo y los tres criterios de aceptación. `fallback-observation.json` registra sesión, hash del manifiesto, prompt, rutas observadas, respuesta saneada y revisión responsable sobre fuente alternativa, motivo comunicado y ausencia de promesa de compresión. Son artefactos independientes del esquema JSON de ahorro v1: no aportan llamadas ni bytes a `ComparisonPair`. Aunque el evaluador de ahorro emita `closed`, la aceptación global queda `pending` si esta observación real falla y `blocked` si no se puede realizar.
