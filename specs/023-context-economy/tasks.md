# Tareas — Economía del contexto

**Estado**: implementadas las Historias 1 y 2. La Historia 3 (medición) queda pendiente.

## Fase 1 — Reglas que no ordenen gastar (Historia 1) ✅

- [X] **T001** Reescribir la sección de orquestación del documento de reglas: la delegación pasa de práctica recomendada a decisión con costo declarado.
- [X] **T002** Importar el documento (`mem docs import rules`); pasa a estado `personalizado` y deja de sobrescribirse.
- [X] **T003** Verificar que el contexto entrega la versión nueva y cero ocurrencias de la instrucción anterior.
- [X] **T004** Retirar el bloque duplicado del archivo de instrucciones de ámbito de usuario (204 → 172 líneas).
- [X] **T005** Actualizar el bloque administrado con las reglas vigentes y deduplicar su marcador.
- [X] **T006** Verificar FR-004 contra el binario: una reinstalación de ámbito global conserva la versión del equipo.

## Fase 2 — Entrega sin repetición (Historia 2) ✅

- [X] **T007** Pruebas en rojo del registro de entregas: alcance de sesión, canales independientes, última entrega gana.
- [X] **T008** Tabla `context_deliveries` con migración idempotente y clave primaria sesión-canal.
- [X] **T009** `RecordContextDelivery` / `LastContextDelivery` en persistencia.
- [X] **T010** Puerto `ports.DeliveryLog`, para que la decisión de suprimir viva en la capa de aplicación.
- [X] **T011** Pruebas en rojo del caso de uso: suprime si coincide, entrega si cambió, entrega si no hay registro, anota lo entregado.
- [X] **T012** Supresión en `PlanContext.Build` con aviso de dónde está el material.
- [X] **T013** `DeliveryLogRepository`, que resuelve la sesión activa en cada llamada.
- [X] **T014** Cablear el composition root y las tres rutas de entrega: CLI, MCP y planificación.
- [X] **T015** `--full` para recuperar el material completo tras una compactación (FR-010).
- [X] **T016** Medir contra el binario: 48.856 → 4.231 bytes, 11.156 tokens de ahorro (91 %).

## Fase 3 — Medición (Historia 3) ⏳ pendiente

- [ ] **T017** Informe de consumo desglosado por canal con la porción duplicada (FR-013 a FR-015).
- [ ] **T018** Separar en el informe el material que entrega la memoria del que consume el trabajo delegado (FR-016).
- [ ] **T019** Política de retención del registro de entregas (FR-018).

## Cierre parcial

**Lo medido**: en este repositorio, el contexto de planificación pasó de 12.214 a 1.057 tokens
cuando el contexto general ya se entregó en la misma sesión. El ahorro escala con el tamaño del
historial: en un proyecto recién creado es de unos 162 tokens, y aquí de 11.156.

**Lectura elegida para FR-008, declarada**: cuando el material cambió desde la entrega anterior
se entrega completo, no un diferencial. Un diferencial de un documento en Markdown sería ruidoso
de consumir para un agente y costaría más de lo que ahorra.

**Anomalía de datos encontrada, ajena a esta feature**: la sesión activa de este repositorio
estaba registrada bajo la clave de proyecto `001` y no bajo la suya. Con esa sesión, la
supresión no se aplicaba — correctamente, porque está acotada a la sesión del proyecto
(FR-012). Se resolvió abriendo una sesión con la clave correcta. Conviene revisar de dónde sale
esa fila.

**Por qué la Historia 3 queda pendiente**: la medición sostiene el ahorro en el tiempo, pero el
ahorro ya está aplicado y verificado contra el binario. Es la única de las tres que no cambia el
comportamiento del sistema.
