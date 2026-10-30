# Reducir el coste visible del pack con medición representativa

ID: HCD-002
Tipo: technical
Carril: full
Motivo: Formato del recibo, representatividad del corpus y presupuesto son decisiones interdependientes y afectan el contrato público de pack_build.

## Intención
Reducir el contexto que gomemory entrega al agente por `get_context`, y por las rutas que usan el mismo documento (contexto de arranque por CLI/hook), conservando lo que el agente necesita para continuar el trabajo. Demostrarlo con un corpus y un protocolo fijados antes de medir y los mismos umbrales que HCD-001: reducción neta ≥20%, sin pérdida de calidad y aumento p95 ≤10%. Continuación de HCD-001 (cancelada por FAIL de reducción neta −14,96% en `pack_build`); Q1 reorienta la unidad hacia la ruta con uso real.

## Alcance
Incluido: el documento de `get_context` y sus entregas (MCP `get_context`, `mem context`, contexto de arranque y de después de compactar), su presupuesto, su selección de secciones y entradas, su delta de sesión y su compresión; corpus y protocolo de medición fijados antes de medir; comparación con la referencia `6c747abd`.
- Q1 reabre la decisión Q3 de HCD-001 para `get_context`: el contrato se puede cambiar dentro de lo que acuerden las aclaraciones.
- Se conservan las garantías del protocolo de memoria (recordatorio de protocolo, reglas fijadas, conflictos y HCD activo), `full=true` sin delta y el reinicio tras compactar.
Excluido: `pack_build` (el fix de aislamiento entre proyectos y sacar el recibo de su salida por defecto van aparte, como Vía rápida); `search_memories`, `get_memory` y `search_code`; salidas externas de terminal u otros MCP; Entroly; cambiar los umbrales después de observar resultados.

## Base observada
- HCD-001 (cancelada): en `pack_build`, el recibo visible aumentó el coste un 14,96%. Su corpus no tensaba el presupuesto y sus casos recovery no producían omisiones.
- Uso real (`usage_records` de todos los proyectos locales, consulta en solo lectura del 2026-10-29; contador local, no tokens facturados). `build_context`: 10 466 llamadas (CLI 9 653, MCP 689, hook 124). Contexto completo, mediana/p95: CLI 27 968/61 336, MCP 37 591/271 322. Emitido, mediana/p95: CLI 4 834/5 920, MCP 4 720/5 841, hook 4 201/5 840. En total, 49,1 M tokens emitidos. `build_pack` solo tiene 14 llamadas.
- Alcance por agente: el documento lo construye gomemory y llega igual a Claude Code (hooks y MCP), Codex (hook `SessionStart` y MCP; `codex exec` solo por MCP) y OpenCode (plugin y MCP). `usage_records` no guarda el agente, así que la distribución observada es por canal. Lo que cada host hace después con el texto queda sin verificar.
- La entrega actual ya recorta la memoria completa unas 6 veces mediante presupuesto, índice y delta. La reducción exigida debe salir del contenido emitido (~4,8k por llamada) sin perder lo que el agente necesita.

## Normas aplicables
Constitución 2.2.1 (revisión `521cfe46f409`): §3 (capas y ports: la selección y el presupuesto del contexto viven en `application/usecases`, y los bordes CLI/MCP/hook solo formatean), §9 y §20 (configuración y secretos: el corpus es sintético, sin datos ni credenciales reales), §10 (pruebas de comportamiento y contrato), §18 (documento HCD único) y §25.2 (composition root existente). Playbook (revisión `fd576035ef3f`): HCD-FLOW-02 (aclarar antes de inferir), HCD-FLOW-03 (aprobación citada), HCD-DEF-01, HCD-TRACE-01 (criterios observables), HCD-EVIDENCE-01 (evidencia vigente), HCD-TDD-01 (rojo antes del verde) y HCD-CLEAN-01–04.

## ¿Qué pasa si no lo hacemos?
Cada sesión y cada llamada a `get_context` sigue costando ~4,8k tokens de contexto (49 M acumulados en los registros locales), aunque el agente no use buena parte. Además, no habría forma reproducible de saber si un cambio en el contexto de arranque hace perder información necesaria. Es la ruta con más uso real, así que es donde una mejora tiene impacto.

## Alternativa más barata
Bajar el presupuesto por defecto del contexto de arranque (un ajuste de configuración) reduciría bytes de inmediato, pero sin medición no hay forma de saber qué se pierde. Por eso no cumple «sin pérdida de calidad». El fix de aislamiento y el recibo de `pack_build` se resuelven aparte (Q1).

## Historias
### US1. Recibir un contexto de arranque más pequeño sin perder lo necesario
Como agente que empieza o retoma una sesión, quiero que `get_context` y el contexto de arranque entreguen menos texto, para gastar menos contexto sin perder decisiones, reglas ni el trabajo activo.
- C-01: Dado el corpus fijado, cuando se entrega el contexto con la mejora, entonces el total de bytes UTF-8 entregados, recuperaciones incluidas, es al menos un 20% menor que en la referencia `6c747abd`.
- C-02: Dado cualquier caso del corpus, cuando se entrega el contexto, entonces siguen presentes el recordatorio de protocolo, las reglas fijadas, los conflictos sin resolver y el HCD activo, y se cumplen los criterios de calidad del caso. Según Q2, la calidad exige ambas condiciones: los literales esperados del caso llegan visibles, y todo lo omitido o resumido se recupera íntegro por ref o `get_memory`, con esa recuperación contada en el coste.
- C-03: Dado un destinatario con `full=true` o una sesión recién compactada, cuando pide contexto, entonces recibe la evidencia completa, sin delta.

### US2. Medir con un corpus que represente el uso real
Como Implementador, quiero un corpus y un protocolo fijados antes de medir, calibrados con el uso real de `get_context`, para que el veredicto no dependa de casos que no ejercitan lo que dicen medir.
- C-04: Dado el corpus, cuando se ejecuta la referencia, entonces el tamaño emitido por caso queda en el rango observado en uso real (mediana ~4,8k tokens; p95 ~5,9k). Según Q3, el corpus es sintético y versionado: imita la forma del uso real (memorias por tipo, longitudes, reglas fijadas, HCD activo) y no contiene datos de proyectos reales.
- C-05: Dado que el corpus y el protocolo quedaron fijados, cuando se observan los resultados, entonces no se modifican casos, presupuesto ni umbrales y los fallos se conservan.

### US3. Decidir con un veredicto que cumpla los umbrales acordados
Como Implementador, quiero comparar la referencia y la mejora sobre el corpus, para adoptarla solo si cumple los umbrales.
- C-06: Dadas 100 observaciones por brazo, cuando se calcula el veredicto, entonces la reducción neta es ≥20%, no hay pérdida de calidad y el aumento p95 es ≤10%; si no, se declara FAIL o inconcluso sin adoptar la mejora.

## Aclaraciones
- [x] Q1. ¿La unidad sigue centrada en pack_build (uso real bajo: 14 llamadas), se reorienta a get_context (10 466 llamadas; reabre Q3 de HCD-001), o se resuelve con las alternativas más baratas (fix de aislamiento + recibo fuera de la salida por defecto) sin medición nueva? → Reorientar a get_context: medir y reducir el contexto que recibe el agente por get_context (reabre Q3 de HCD-001); el fix de aislamiento entre proyectos y sacar el recibo de la salida por defecto de pack_build van aparte como Vía rápida y se publican antes.
- [x] Q2. ¿Qué define «sin pérdida de calidad» en get_context: literales esperados por caso (decisiones, reglas, tarea activa presentes), recuperabilidad íntegra de lo omitido, o ambos? → Ambos: cada caso declara literales que deben llegar visibles (decisiones, reglas fijadas, tarea/HCD activo) y todo lo omitido o resumido debe recuperarse íntegro por ref o get_memory, contando esa recuperación en el coste.
- [x] Q3. ¿El corpus de get_context es sintético calibrado a los tamaños de uso real (mediana ~4,8k, p95 ~5,9k tokens emitidos) o se deriva de memorias reales anonimizadas? → Sintético calibrado: memorias sintéticas con la forma del uso real (memorias por tipo, longitudes, reglas fijadas, HCD activo) y tamaños emitidos en el rango observado (mediana ~4,8k, p95 ~5,9k tokens); reproducible, versionable y sin datos de proyectos reales.
- [x] Q4. Palanca de reducción de get_context (cambia su contenido): ¿índice de entradas (título + get_memory; reglas fijadas, HCD activo y conflictos íntegros; ~44%), extracto de 80 caracteres (~25%) o extracto de 50 caracteres (~32%)? → Extracto de 80 caracteres: las entradas de get_context pasan de un extracto de 200 a uno de 80 caracteres con puntero get_memory; las reglas fijadas, el HCD activo y los conflictos se mantienen íntegros (~25% estimado).

## Slices
### VS-01. Medir el contexto actual con un corpus que represente el uso real
Tareas: 1, 2, 3, 4, 5
Criterios: C-04, C-05
Resultado observable: un corpus sintético versionado, calibrado a la forma y los tamaños del uso real de `get_context`, y un protocolo fijado antes de medir. Con ambos se captura la referencia `6c747abd` por las entregas reales (MCP `get_context`, `mem context`, contexto de arranque y de después de compactar), con literales y recuperaciones por caso.
Comprobación de entrega: los tamaños emitidos de la referencia caen en el rango observado (mediana ~4,8k, p95 ~5,9k tokens locales); cada caso declara sus literales visibles y sus recuperaciones; las capturas quedan identificadas por huella antes de cualquier cambio de producción.
Límites: corpus sin datos ni credenciales reales (Q3); no se cambia código de producción en este slice.
Dependencias: ninguna.

### VS-02. Recibir un contexto reducido sin perder lo obligatorio ni lo recuperable
Tareas: 6, 7, 8, 9
Criterios: C-02, C-03
Resultado observable: `get_context` y las entregas que comparten su documento emiten menos texto. Siguen presentes el recordatorio de protocolo, las reglas fijadas, los conflictos y el HCD activo, junto con los literales del caso; todo lo omitido se recupera íntegro por ref o `get_memory`, y `full=true` y la sesión recién compactada reciben la evidencia completa sin delta.
Comprobación de entrega: pruebas de contrato por cada entrega (MCP, CLI y hook) y observación del binario servido con un caso del corpus.
Límites: la palanca de reducción se elige analizando la referencia de VS-01, no por suposición, y si cambia el contrato de `get_context` se aclara primero; la lógica vive en `application/usecases` y los bordes solo formatean (Constitución §3).
Dependencias: VS-01, para elegir la palanca con datos y medir frente a la referencia preservada.

### VS-03. Decidir con un veredicto que cumpla los umbrales acordados
Tareas: 10, 11
Criterios: C-01, C-06
Resultado observable: el informe del documento compara la referencia y la mejora sobre el mismo corpus, con 100 observaciones por brazo, resultados por caso, reducción neta con recuperaciones, calidad y p95. Declara PASS solo si se cumplen los tres umbrales; si no, FAIL o inconcluso, sin adoptar la mejora.
Comprobación de entrega: recalcular el veredicto desde las capturas conforme al protocolo de VS-01.
Límites: no se ajustan casos, presupuesto ni umbrales tras observar resultados (C-05).
Dependencias: VS-01 y VS-02.

Árbol de incrementos y dependencias:
```text
HCD-002: reducir get_context con beneficio verificable
├─ VS-01: medir con corpus representativo y referencia fijada
│  └─ VS-02: entregar contexto reducido con garantías (dep: VS-01)
│     └─ VS-03: veredicto con umbrales (dep: VS-01, VS-02)
```

## Checklist
<!-- Los nombres TestHCD002* son pruebas que se crearán durante la implementación: todavía no existen ni son evidencia. Cada comprobación debe mostrar el nombre de la prueba ejecutada; un filtro sin coincidencias no cuenta como PASS. -->
- [x] 1. Fijar en este documento el perfil de calibración del uso real de `get_context`: llamadas, tamaños completos y emitidos por canal, número de memorias por tipo, longitudes, reglas fijadas y HCD activo → inspección manual de cifras agregadas obtenidas en solo lectura, sin copiar contenido de memorias reales (C-04).
- [x] 2. Crear el corpus sintético `tests/fixtures/hcd002/corpus.json` con 20 casos, cinco por categoría: arranque, continuación tras compactación, destinatario con `full=true` y recuperación de lo omitido. Cada caso declara sus literales visibles y sus recuperaciones (dep: 1) → prueba `TestHCD002CorpusContract`: IDs únicos, huellas de las fuentes, literales y recuperaciones declarados, ausencia de patrones de credenciales y tamaño de proyecto dentro del perfil de 1 (C-04, C-05).
- [x] 3. Fijar en este documento el protocolo de medición: unidad bytes UTF-8 visibles con recuperaciones, entregas MCP/CLI/hook ejercitadas por categoría, política de recuperación, cinco repeticiones por caso, reloj monotónico, p95 por rango más cercano y fórmula de reducción neta (dep: 2) → inspección del protocolo antes de cualquier captura; fallos y tiempos agotados se conservan (C-05).
- [x] 4. Crear el ejecutor aislado `scripts/hcd002_evaluate.py` que capture entregas y recuperaciones completas por caso según el protocolo (dep: 3) → prueba de contrato `TestHCD002EvaluationCapture` con contabilidad UTF-8, p95, fallos conservados y frontera del modelo marcada como no verificada (C-05).
- [x] 5. Capturar la referencia `6c747abd` con el ejecutor sobre el corpus fijado, antes de cambiar código de producción (dep: 4) → observación de 100 observaciones con huellas de binario, corpus y capturas; los tamaños emitidos caen en el perfil de 1 (C-04, C-05).
- [x] 6. Fijar en este documento la palanca de reducción, elegida a partir del desglose de bytes por sección de la referencia (dep: 5) → inspección del desglose y del ahorro estimado; si la palanca cambia el contrato de `get_context`, abrir una aclaración antes de la tarea 7 (C-02).
- [x] 7. Implementar la palanca en `application/usecases`, conservando el recordatorio de protocolo, las reglas fijadas, los conflictos, el HCD activo y los literales del caso (dep: 6) → prueba `TestHCD002ReducedContext` con el rojo observado antes del cambio: menos bytes en un caso del corpus y elementos obligatorios presentes (C-02).
- [x] 8. Garantizar que todo lo omitido o resumido por la palanca se recupera íntegro por ref o `get_memory` (dep: 7) → prueba `TestHCD002OmittedRecovery`: el original coincide por huella y ninguna referencia carece de original disponible (C-02).
- [x] 9. Preservar `full=true` sin delta y el reinicio tras compactar con la palanca activa (dep: 7) → prueba `TestHCD002FullAndCompaction` sobre MCP `get_context`, `mem context` y el hook de arranque, más la observación del binario servido con un caso del corpus (C-03).
- [x] 10. Capturar el brazo de mejora con el ejecutor sobre el mismo corpus y protocolo (dep: 5, 8, 9) → observación de 100 observaciones con huellas, entregas, recuperaciones, calidad y latencia por caso; los fallos se conservan (C-01).
- [x] 11. Publicar en este documento el veredicto por caso y agregado (dep: 10) → recalcular desde las capturas la reducción neta (≥20%), la calidad (cero pérdidas) y el p95 (≤10%); declarar FAIL o inconcluso si no se cumplen, sin ajustar casos, presupuesto ni umbrales (C-01, C-06).

Árbol de tareas atómicas:
```text
HCD-002
├─ VS-01: 1 → 2 → 3 → 4 → 5
├─ VS-02: 6 → 7 → {8 ∥ 9} (dep: 5)
└─ VS-03: 10 → 11 (dep: 5, 8, 9)
```
Comandos: `go test <paquete> -run '^TestHCD002Nombre$' -count=1 -v`. Paquetes: `tests/contract` para 2 y 4, `application/usecases` para 7 y 8, y `adapters/primary/cli` para 9. Las observaciones manuales se registran con `mem hcd verify --manual`.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-30 00:55 · tarea 1 · manual · caducada
      Perfil de calibración fijado en el documento a partir de consultas en solo lectura sobre usage_records y memories de 64 proyectos locales (10466 llamadas build_context). Solo recuentos, longitudes y tamaños emitidos por canal y por franja de tamaño; no se copió contenido. Caso representativo: ~376 memorias curadas, emitido ~4,8k con un tope de ~5,8k tokens; cola anómala en la franja 10-100 (p95 14385).
- 2026-10-30 00:58 · tarea 2 · verde · caducada · `go test ./tests/contract -run '^TestHCD002CorpusContract$' -count=1 -v` (salida 0)
      === RUN   TestHCD002CorpusContract
      --- PASS: TestHCD002CorpusContract (0.74s)
      PASS
      ok  	mem/tests/contract	1.009s
- 2026-10-30 00:58 · tarea 2 · manual · caducada
      Corpus generado por scripts/hcd002_corpus.py (semilla 20261029), SHA256 1e32011852fe29abcd7562bbc6516cb1c05eb21f62628f951ce1e0ef9e6c23f5, 1,07 MB. Representativo: 377 curadas + 376 checkpoints, mediana 1014 y p95 4041 caracteres (perfil 986/3762), una regla fijada gomemory:work-rules. Cola: 61 curadas largas (mediana 2312, p95 9549). Validez de la prueba comprobada por mutación: un secreto inyectado falla por el patrón de credenciales y un contenido alterado falla por la reproducibilidad. Sin rojo previo: el test valida un artefacto nuevo.
- 2026-10-30 01:00 · tarea 2 · verde · caducada · `go test ./tests/contract -run '^TestHCD002CorpusContract$' -count=1 -v` (salida 0)
      === RUN   TestHCD002CorpusContract
      --- PASS: TestHCD002CorpusContract (0.69s)
      PASS
      ok  	mem/tests/contract	0.966s
- 2026-10-30 01:00 · tarea 2 · manual · caducada
      Corpus final antes de medir, SHA256 b55d8bc464a5a5e9fb561f3d79a1d04c223d7e8df1dfef9978aafcd16ba7ae20, generado por scripts/hcd002_corpus.py (semilla 20261029). Correcciones de calibración previas a cualquier medición: fechas de checkpoints y curadas intercaladas (get_context solo lee las 100 más recientes), 503 relaciones related (perfil real), títulos con forma de frase y regla fijada de ~1200 caracteres. Sonda con el binario de referencia y una unidad HCD activa: representativo ≈5364 tokens y cola ≈4828, dentro del rango real p5-p95 de 3990-5855. Validez de la prueba comprobada por mutación (secreto inyectado y contenido alterado fallan).
- 2026-10-30 01:00 · tarea 3 · manual · caducada
      Protocolo fijado en el documento antes de cualquier captura: corpus b55d8bc4 (calibrado antes de medir), dos brazos (6c747abd y árbol de trabajo) con nivel max, preparación aislada fuera de la latencia (import y HCD activo), entregas por categoría (session-start; context, post-compact, context; context y context --full; context, search y get), calidad Q2 (literales visibles y HCD activo, más recuperación byte a byte por SHA-256), bytes UTF-8 con recuperaciones, 20x5 observaciones, reloj monotónico, p95 por rango más cercano, fallos y timeouts conservados, umbrales sin cambios y frontera en stdout de la CLI.
- 2026-10-30 01:02 · tarea 4 · verde · caducada · `go test ./tests/contract -run '^TestHCD002EvaluationCapture$' -count=1 -v` (salida 0)
      === RUN   TestHCD002EvaluationCapture
      --- PASS: TestHCD002EvaluationCapture (0.12s)
      PASS
      ok  	mem/tests/contract	0.376s
- 2026-10-30 01:02 · tarea 4 · manual · caducada
      Ejecutor scripts/hcd002_evaluate.py con self-test (bytes UTF-8, p95, fallos conservados, literales y HCD activo obligatorios, búsqueda del id en la tabla de mem search, frontera no verificada). La prueba de humo de cableado (no es medición) contra la referencia detectó un error del ejecutor: mem search imprime una tabla con títulos truncados; se corrigió y la recuperación devuelve el original por huella. También mostró un comportamiento real del producto que NO se ajustó: el hook session-start entrega un contexto compacto (~6,6 KB) sin las decisiones recientes, así que arranque fallará en la referencia; el protocolo fijado lo mide tal cual y la mejora no puede fallar más.
- 2026-10-30 01:04 · tarea 5 · manual · caducada
      Referencia 6c747abd capturada antes de cualquier cambio de producción para HCD-002: binario 89b872d1, corpus b55d8bc4, 100 observaciones, 3189195 bytes, 0 errores, p95 206,0 ms, 30 fallos de calidad conservados (arranque 25 y recuperacion-03 5, ambos por comportamiento del producto). Captura hcd002-baseline.json SHA256 57d03ae8. Tamaño por contexto (~22,2 KB, ~5,4k tokens) dentro del perfil real.
- 2026-10-30 01:06 · tarea 6 · manual · caducada
      Palanca fijada tras el desglose de la referencia (extractos 50,3% de los bytes; títulos 21,7%) y la simulación sobre sus entregas (índice ~44,2%, extracto 50 ~32,1%, extracto 80 ~25,3%, extracto 120 ~16,2%). Como cambia el contrato de get_context, se abrió la aclaración Q4: la persona eligió «Extracto de 80 caracteres» y aprobó la revisión 7628ec3b28de. Reglas fijadas, HCD activo y conflictos se mantienen íntegros.
- 2026-10-30 01:07 · tarea 7 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD002ReducedContext$' -count=1 -v` (salida 1)
       contrato motivo 6 del contrato motivo 6 del contrato motivo 6 del contrato motivo…"
          hcd002_reduced_context_test.go:56: extracto de 199 caracteres (máximo 80 más el corte): "decisión 5 á漢字: motivo 5 del contrato motivo 5 del contrato motivo 5 del contrato motivo 5 del contrato motivo 5 del contrato motivo 5 del contrato motivo 5 del contrato motivo 5 del contrato motivo…"
          hcd002_reduced_context_test.go:56: extracto de 199 caracteres (máximo 80 más el corte): "decisión 4 á漢字: motivo 4 del contrato motivo 4 del contrato motivo 4 del contrato motivo 4 del contrato motivo 4 del contrato motivo 4 del contrato motivo 4 del contrato motivo 4 del contrato motivo…"
          hcd002_reduced_context_test.go:56: extracto de 199 caracteres (máximo 80 más el corte): "decisión 3 á漢字: motivo 3 del contrato motivo 3 del contrato motivo 3 del contrato motivo 3 del contrato motivo 3 del contrato motivo 3 del contrato motivo 3 del contrato motivo 3 del contrato motivo…"
          hcd002_reduced_context_test.go:56: extracto de 199 caracteres (máximo 80 más el corte): "decisión 2 á漢字: motivo 2 del contrato motivo 2 del contrato motivo 2 del contrato motivo 2 del contrato motivo 2 del contrato motivo 2 del contrato motivo 2 del contrato motivo 2 del contrato motivo…"
          hcd002_reduced_context_test.go:56: extracto de 199 caracteres (máximo 80 más el corte): "decisión 1 á漢字: motivo 1 del contrato motivo 1 del contrato motivo 1 del contrato motivo 1 del contrato motivo 1 del contrato motivo 1 del contrato motivo 1 del contrato motivo 1 del contrato motivo…"
          hcd002_reduced_context_test.go:56: extracto de 199 caracteres (máximo 80 más el corte): "decisión 0 á漢字: motivo 0 del contrato motivo 0 del contrato motivo 0 del contrato motivo 0 del contrato motivo 0 del contrato motivo 0 del contrato motivo 0 del contrato motivo 0 del contrato motivo…"
      --- FAIL: TestHCD002ReducedContext (0.02s)
      FAIL
      FAIL	mem/application/usecases	0.176s
      FAIL
- 2026-10-30 01:08 · tarea 7 · verde · caducada · `go test ./application/usecases -run '^TestHCD002ReducedContext$' -count=1 -v` (salida 0)
      === RUN   TestHCD002ReducedContext
      --- PASS: TestHCD002ReducedContext (0.02s)
      PASS
      ok  	mem/application/usecases	0.198s
- 2026-10-30 01:09 · tarea 8 · verde · caducada · `go test ./application/usecases -run '^TestHCD002OmittedRecovery$' -count=1 -v` (salida 0)
      === RUN   TestHCD002OmittedRecovery
      --- PASS: TestHCD002OmittedRecovery (0.02s)
      PASS
      ok  	mem/application/usecases	0.200s
- 2026-10-30 01:09 · tarea 8 · manual · caducada
      Sin rojo natural: el puntero get_memory de acota ya existía. Validez comprobada por mutación temporal (puntero a m.ID+1000): la prueba falla con 'el puntero ... no resuelve'; código restaurado con diff idéntico. Cubre seis tipos de memoria con contenido multibyte y comparación por SHA-256.
- 2026-10-30 01:10 · tarea 9 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD002FullAndCompaction$' -count=1 -v` (salida 0)
      === RUN   TestHCD002FullAndCompaction
      --- PASS: TestHCD002FullAndCompaction (0.04s)
      PASS
      ok  	mem/adapters/primary/cli	0.338s
- 2026-10-30 01:10 · tarea 9 · manual · vigente
      Sin rojo natural: full=true sin delta y el reinicio tras compactar ya existían; la prueba los fija con la palanca activa sobre el hook de arranque, MCP get_context y la ruta de mem context (deliverContextDocFull), con el registro de bloques real. Binario servido a4bac2b1 frente a la referencia 89b872d1, un caso por categoría: compactacion-01 -26,1%, full-01 -31,8% y recuperacion-01 -28,7% con calidad en ambos; arranque-01 +46,6% de bytes, pero pasa de fallar a cumplir la calidad, porque el hook ya incluye las decisiones dentro de su tope.
- 2026-10-30 01:12 · tarea 10 · manual · vigente
      Brazo de mejora capturado con el ejecutor de la tarea 4 sobre el corpus b55d8bc4 y el binario a4bac2b143213773e30895d68e2866d3e9ce1dae0bb1bb8a10a0dc46fa9136d1: 100 observaciones, 2395755 bytes con recuperaciones, 0 errores, 5 fallos de calidad (recuperacion-03, igual que la referencia), p95 191,2 ms. Referencia recapturada a continuación: 3189195 bytes (idéntica a la tarea 5), 30 fallos, p95 208,3 ms. Capturas hcd002-improved.json SHA256 ff96ad3a10c1dc5fbe10d675e5501b60e4020697d2226c816ed2164e050bfea3 y hcd002-baseline-rerun.json SHA256 c0936fed46b90ca81f57a3cfd772b1206212022a4e371ac1d6f45b2b145e513b. Frontera: stdout de la CLI.
- 2026-10-30 01:12 · tarea 11 · manual · vigente
      Veredicto publicado y recalculado desde las capturas según el protocolo de la tarea 3: reducción neta 24,88% (2395755 frente a 3189195 bytes UTF-8 con recuperaciones, umbral >=20%); calidad sin pérdidas (0 casos que fallen solo en la mejora y 5 de arranque que dejan de fallar; recuperacion-03 falla en ambos brazos); p95 -8,22% (191,2 frente a 208,3 ms, umbral <=10%). PASS. Casos, presupuesto y umbrales sin ajustar tras observar. Límites declarados: frontera en stdout de la CLI, corpus sintético y aumento de bytes del arranque.
- 2026-10-30 01:14 · tarea 1 · manual · vigente
      Perfil de calibración del uso real fijado en el documento: 64 proyectos, 10466 llamadas build_context; emitido mediana/p95 por canal (CLI 4834/5920); ~376 memorias curadas por llamada; mezcla de tipos y longitudes; solo agregados en lectura. (re-registro con la huella final, tras implementar 7-9; observación no repetida, sigue vigente porque no depende del código cambiado)
- 2026-10-30 01:14 · tarea 2 · verde · vigente · `go test ./tests/contract -run '^TestHCD002CorpusContract$' -count=1 -v` (salida 0)
      === RUN   TestHCD002CorpusContract
      --- PASS: TestHCD002CorpusContract (0.72s)
      PASS
      ok  	mem/tests/contract	0.977s
- 2026-10-30 01:14 · tarea 3 · manual · vigente
      Protocolo fijado antes de medir en el documento (corpus b55d8bc4, entregas por categoría, calidad Q2, bytes UTF-8 con recuperaciones, 20x5, p95 por rango más cercano, umbrales). (re-registro con la huella final, tras implementar 7-9; observación no repetida, sigue vigente porque no depende del código cambiado)
- 2026-10-30 01:14 · tarea 4 · verde · vigente · `go test ./tests/contract -run '^TestHCD002EvaluationCapture$' -count=1 -v` (salida 0)
      === RUN   TestHCD002EvaluationCapture
      --- PASS: TestHCD002EvaluationCapture (0.12s)
      PASS
      ok  	mem/tests/contract	0.368s
- 2026-10-30 01:14 · tarea 5 · manual · vigente
      Referencia 6c747abd (binario 89b872d1) capturada antes de cambiar producción: 3189195 bytes, p95 206,0 ms, 30 fallos de calidad del producto; captura 57d03ae8. Recapturada en la tarea 10 con bytes idénticos. (re-registro con la huella final, tras implementar 7-9; observación no repetida, sigue vigente porque no depende del código cambiado)
- 2026-10-30 01:14 · tarea 6 · manual · vigente
      Palanca fijada con el desglose de la referencia (extractos 50,3% de los bytes) y elegida por la persona en Q4 (extracto de 80 caracteres), con la revisión 7628ec3b28de aprobada. (re-registro con la huella final, tras implementar 7-9; observación no repetida, sigue vigente porque no depende del código cambiado)
- 2026-10-30 01:14 · tarea 7 · verde · vigente · `go test ./application/usecases -run '^TestHCD002ReducedContext$' -count=1 -v` (salida 0)
      === RUN   TestHCD002ReducedContext
      --- PASS: TestHCD002ReducedContext (0.02s)
      PASS
      ok  	mem/application/usecases	0.209s
- 2026-10-30 01:14 · tarea 8 · verde · vigente · `go test ./application/usecases -run '^TestHCD002OmittedRecovery$' -count=1 -v` (salida 0)
      === RUN   TestHCD002OmittedRecovery
      --- PASS: TestHCD002OmittedRecovery (0.01s)
      PASS
      ok  	mem/application/usecases	0.185s
- 2026-10-30 01:14 · tarea 9 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD002FullAndCompaction$' -count=1 -v` (salida 0)
      === RUN   TestHCD002FullAndCompaction
      --- PASS: TestHCD002FullAndCompaction (0.04s)
      PASS
      ok  	mem/adapters/primary/cli	0.337s

## Siguiente
Cerrada con aprobación sobre la revisión 353e3da20b06.

## Registro de implementación

### Perfil de calibración del uso real (tarea 1)
Consulta en solo lectura del 2026-10-29 sobre `usage_records` y `memories` de los proyectos locales. Solo se extrajeron recuentos y longitudes; no se copió contenido de memorias. Los tokens son del contador local, no tokens facturados.

| Dimensión | Valor |
|---|---|
| Proyectos con uso de `get_context` | 64; 10 466 llamadas (CLI 9 653, MCP 689, hook 124) |
| Emitido por canal, mediana/p95 | CLI 4 834/5 920; MCP 4 720/5 841; hook 4 201/5 840 |
| Memorias curadas (sin checkpoints), ponderadas por llamada | mediana 376; p95 405 |
| Emitido por tamaño de proyecto (memorias curadas) | 0–10: 481 llamadas, 185/824 · 10–100: 1 701, 1 461/14 385 · 100–400: 6 287, 4 774/5 870 · ≥400: 1 997, 5 202/5 791 |
| Mezcla de tipos (todas las memorias) | checkpoint 49,5% · decision 14,0% · bugfix 9,6% · discovery 9,4% · architecture 5,0% · preference 4,8% · pattern 4,5% · learning 3,2% |
| Longitud del contenido | mediana 986, p95 3 762 caracteres; 595 memorias con `topic_key` (reglas o temas fijados) |

Lectura para el corpus: el caso representativo es un proyecto de unas 376 memorias curadas, con la mezcla de tipos y las longitudes anteriores, cuyo contexto emitido ronda los 4,8k tokens y choca con un tope cercano a 5,8k. Los proyectos pequeños (<10 memorias) apenas pesan en el coste. En la franja de 10–100 aparece una cola anómala (p95 de 14 385) que el corpus debe incluir como caso, no promediar.

### Protocolo fijado antes de medir (tarea 3)
- **Corpus:** `tests/fixtures/hcd002/corpus.json`, SHA-256 `b55d8bc464a5a5e9fb561f3d79a1d04c223d7e8df1dfef9978aafcd16ba7ae20`, generado por `scripts/hcd002_corpus.py`. Se corrigió su calibración antes de cualquier medición: fechas intercaladas, 503 relaciones, títulos con forma de frase y regla fijada realista. Las secciones que dependen del entorno (grafo de código, anclas, estilo de respuesta, instincts globales) quedan fuera del corpus porque varían por instalación.
- **Brazos:** referencia (binario construido desde `git archive` de `6c747abd`) y mejora (binario del árbol de trabajo). Ambos usan el mismo corpus, el nivel de compresión `max` y los mismos pasos.
- **Preparación por observación** (fuera de la latencia): almacén aislado (`GOMEMORY_DATA_HOME` y `HOME` temporales), repositorio git temporal, `session start`, `mem import` del proyecto del caso y `mem hcd start` de una unidad fast sintética, para que exista un HCD activo.
- **Entregas por categoría**, contadas como visibles:
  - arranque: `mem hook session-start` con el payload de arranque por stdin;
  - compactacion: `mem context`, después `mem hook post-compact` y después `mem context`;
  - full: `mem context` y después `mem context --full`;
  - recuperacion: `mem context` y, por cada original declarado, `mem search "<título>"` y `mem get <id>`.
- **Calidad (Q2: ambas condiciones):**
  - Visibilidad: todos los literales `required_visible` del caso aparecen en la entrega final de la categoría (la salida de arranque, la salida post-compact junto con el `context` siguiente, la salida `--full` o el primer `context`), y además aparece el encabezado `## HCD activo`.
  - Recuperación: cada original declarado aparece íntegro, byte a byte, en la salida de `mem get` y coincide con su SHA-256.
  - Si un caso falla en algo, se marca como fallo.
- **Unidad y fórmula:** bytes UTF-8 de todo stdout visible, incluidos avisos, marcadores y todas las recuperaciones. Reducción neta = `1 − Σ bytes_mejora / Σ bytes_referencia` sobre las 100 observaciones. El contador de tokens es secundario y aproximado; no son tokens facturados.
- **Ejecución:** 20 casos × 5 repeticiones por brazo, con estado nuevo en cada observación. Los brazos se ejecutan uno tras otro y la referencia se recaptura junto a la mejora al comparar.
- **Latencia:** reloj monotónico desde la primera entrega hasta la última recuperación. p95 = elemento 95 de las 100 duraciones ordenadas (rango más cercano). Aumento = `p95_mejora / p95_referencia − 1`.
- **Fallos:** un error de proceso o un timeout (30 s por comando) cuenta como fallo de calidad y se conserva en el informe con su duración. Una muestra ausente o `p95_referencia = 0` vuelven el resultado inconcluso.
- **Umbrales:** reducción neta ≥20%, cero fallos de calidad adicionales respecto de la referencia (los casos de la referencia que fallen se registran y la mejora no puede fallar más) y aumento p95 ≤10%. No se ajustan después de observar resultados.
- **Frontera:** se observa stdout de la CLI, no la entrada del modelo de cada agente.

### Referencia capturada (tarea 5)
Binario construido desde `git archive` de `6c747abd` (SHA-256 `89b872d1278aba18d5abe491f62d11a1eb3bb9d03af1226091cb50e986438eec`), corpus `b55d8bc4…`. Captura local `hcd002-baseline.json` en el directorio temporal de la sesión (SHA-256 `57d03ae8eaf70efb673225fde8c446bb174c7e613c30dd7ba32c7a838fc4171b`). Resultado: 100 observaciones, 3 189 195 bytes, 0 errores de proceso, p95 de 206,0 ms y 30 fallos de calidad, que se conservan.

| Categoría | Bytes (25 obs.) | Peso | Calidad |
|---|---|---|---|
| arranque | 171 820 | 5,4% | 0/25 |
| compactacion | 1 320 920 | 41,4% | 25/25 |
| full | 1 087 080 | 34,1% | 25/25 |
| recuperacion | 609 375 | 19,1% | 20/25 |

Los fallos los causa el producto, no el ejecutor, y no se corrigen en el corpus:
- arranque: `hook session-start` entrega un contexto compacto (~6,6 KB) sin las decisiones recientes.
- recuperacion-03: `mem search` comprime su tabla y deja la fila buscada tras una marca `⟦mem⟧ … ref=` que el protocolo no sigue.

Ambos brazos se tratan igual.

### Palanca de reducción (tarea 6)
Desglose de la referencia: los extractos de las entradas ocupan el 50,3% de los bytes entregados y los títulos el 21,7%. Por sección: Aprendizajes 19,0%, Decisiones Técnicas 18,4%, Sinapsis 12,5%, Bugfixes 11,9%, Patrones 9,4%, Preferencias 8,9%, Reglas fijadas 6,1%, Decisiones de Arquitectura 5,8%, Actividad 3,6%.

Estimaciones, simulando la sustitución de las entradas sobre las entregas de la referencia:
- índice de entradas (título y puntero, con la regla fijada íntegra): ~44,2%;
- extracto de 50 caracteres: ~32,1%;
- extracto de 80 caracteres: ~25,3%;
- extracto de 120 caracteres: ~16,2%.

El `IndexMode` existente colapsa también la regla fijada, lo que incumple C-02, así que no se usa tal cual. Cualquier palanca cambia el contenido de `get_context`; la elección se registra como aclaración (Q4) antes de la tarea 7.

### Brazo de mejora (tarea 10)
Binario del árbol de trabajo `a4bac2b143213773e30895d68e2866d3e9ce1dae0bb1bb8a10a0dc46fa9136d1`, mismo corpus y protocolo. Captura `hcd002-improved.json` (SHA-256 `ff96ad3a10c1dc5fbe10d675e5501b60e4020697d2226c816ed2164e050bfea3`). La referencia se recapturó justo después (`hcd002-baseline-rerun.json`, SHA-256 `c0936fed46b90ca81f57a3cfd772b1206212022a4e371ac1d6f45b2b145e513b`) y dio los mismos 3 189 195 bytes que en la tarea 5. Límite que se conserva: los brazos se ejecutan uno tras otro, sin intercalar, y se observa stdout de la CLI, no la entrada del modelo de cada agente.

### Veredicto comparativo (tarea 11): **PASS**
| Categoría (5 casos × 5 repeticiones) | Bytes referencia | Bytes mejora | Reducción | Calidad referencia | Calidad mejora |
|---|---|---|---|---|---|
| arranque | 171 820 | 244 880 | −42,5% | 0/25 | 25/25 |
| compactacion | 1 320 920 | 974 320 | 26,2% | 25/25 | 25/25 |
| full | 1 087 080 | 740 480 | 31,9% | 25/25 | 25/25 |
| recuperacion | 609 375 | 436 075 | 28,4% | 20/25 | 20/25 |
| **Agregado (100 observaciones)** | **3 189 195** | **2 395 755** | **24,88%** | 70/100 | 95/100 |

- Reducción neta, recuperaciones incluidas: **24,88%** (umbral ≥20%). Cumple.
- Calidad: ningún caso falla en la mejora sin fallar en la referencia. Los 5 casos de arranque pasan a cumplir, porque con extractos de 80 caracteres el hook de arranque ya incluye las decisiones recientes dentro de su tope de 10 000 caracteres. recuperacion-03 sigue fallando en ambos brazos: `mem search` comprime su tabla y la fila buscada queda tras una marca `⟦mem⟧` que el protocolo no sigue. Umbral «sin pérdida»: cumple.
- Latencia p95: 191,2 ms en la mejora frente a 208,3 ms en la referencia, −8,22% (umbral de aumento ≤10%). Cumple.
- Coste observado: el arranque emite más bytes (+42,5%) porque ahora llena su tope con evidencia útil. Pesa un 5,4% de la referencia y queda absorbido en el agregado.
- Límites de lo demostrado: el corpus es sintético y está calibrado con el uso local; se midió stdout de la CLI, no lo que recibe el modelo de cada agente (Claude Code, Codex, OpenCode); y el corpus no mide del todo las llamadas adicionales a `get_memory` que un agente real pueda hacer por tener extractos más cortos.
- Hallazgo abierto, fuera del alcance de esta unidad: la compresión de la tabla de `mem search` puede ocultar la fila exacta que se busca (recuperacion-03). Propuesta: una unidad `fix` aparte que no comprima la fila cuyo título coincide con la consulta.
