# Feature Specification: Compression Promise Gaps

**Feature Branch**: `036-compression-promise-gaps`

**Created**: 2026-10-06

**Status**: Draft

**Input**: User description: "arma esto una especificación para cerrar brechas (del informe de auditoría de compresión por harness: bimodal, Codex sin rewrite, pack_build +5pp con grafo)"

## Contexto medido (base de la especificación)
Auditoría del 2026-10-06 sobre el binario instalado (memoria [378]):

- El ahorro grande viene de la emisión de contexto (línea base 432099 → emitido 67495, 84.38% en la sesión medida), no del motor de compresión nativa.
- El motor nativo es bimodal: contenidos estructurados grandes y repetitivos se reducen mucho (json 81.5%, listing 92.7%, table 74.9% en 28 usos); el volumen dominante (structural/prose, ~1.1M tokens) apenas se reduce (~0.3%) y degrada al original intacto por diseño.
- El hook de salidas solo reescribe cuando hay ganancia recuperable (más de ~2000 tokens, referencias emitidas): un JSON de 30 elementos no se toca, uno de 60 sí (20k caracteres → ~1k, ref recuperable). Código, prosa en modo salida, lecturas exactas y diagnósticos se preservan intactos.
- Por harness: el asistente de terminal con reescritura de salidas comprime (verificado en vivo); el plugin del editor con hook posterior a herramienta emite reescritura best-effort (verificado que emite, sin garantía de que el runtime la aplique); el asistente sin reescritura no toca salidas por límite de su runtime y solo ahorra vía paquete de contexto y búsqueda de código que comprime en origen.

## Clarifications

### Session 2026-10-06

- Q: ¿En qué superficies debe actualizarse la documentación de ahorro por vía (FR-001)? → A: Guía + manual + textos de ayuda en línea, sin TUI.
- Q: ¿Deben las cifras de ahorro publicadas incluir números concretos con fecha de medición o solo órdenes de magnitud cualitativos? → A: Números medidos con fecha de medición y comando para reproducirlos.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Entender qué ahorro esperar de cada vía (Priority: P1)

La persona lee la documentación de ahorro y entiende, antes de activar nada, que hay tres vías distintas (contexto emitido, paquete de contexto bajo presupuesto, hook de salidas opt-in), qué orden de ahorro suele dar cada una y en qué casos el hook no actúa (contenido pequeño, código, lecturas exactas).

**Why this priority**: Es la brecha 1 del informe. Hoy el mensaje genérico de "ahorro" invita a esperar el 80%+ en todas partes; el valor real está concentrado en la emisión de contexto y en contenidos grandes y repetitivos.

**Independent Test**: Se puede probar solo con la documentación y la ayuda en línea: una persona nueva describe correctamente qué ahorro esperar de cada vía y cuándo el hook deja la salida intacta, sin leer el código.

**Acceptance Scenarios**:

1. **Given** la guía de instalación y el manual, **When** la persona busca "cuánto ahorra", **Then** encuentra una tabla por vía con el orden de magnitud medido y cómo reproducir la medición.
2. **Given** la ayuda del comando de ahorro, **When** la persona la lee, **Then** distingue el ahorro por compresor del ahorro por emisión de contexto y no los confunde.
3. **Given** un contenido pequeño o código fuente, **When** la persona pregunta si el hook lo comprimirá, **Then** la documentación responde que sale intacto y por qué (preservar lo editable y lo literal).

---

### User Story 2 - Diagnóstico veraz del hook en el editor con plugin (Priority: P2)

La persona ejecuta el diagnóstico y el estado del hook de salidas para el editor con plugin refleja lo medido: el hook emite reescritura best-effort y la sustitución final depende del runtime, en lugar de declarar "no soportado" o "sin verificar".

**Why this priority**: Es la brecha 2 del informe. El diagnóstico contradice el comportamiento verificado (el hook sí emite) y genera un reporte de problema inexistente.

**Independent Test**: Se puede probar solo con el diagnóstico: con el plugin instalado, el estado del hook describe emisión best-effort y no aparece como problema a corregir.

**Acceptance Scenarios**:

1. **Given** el plugin instalado, **When** la persona ejecuta el diagnóstico, **Then** el hook de salidas del editor aparece como activo/emitiendo con su advertencia de best-effort, no como fallo.
2. **Given** una salida grande y repetitiva en ese editor, **When** el hook la procesa, **Then** emite la versión comprimida con referencia recuperable igual que en el asistente de terminal.

---

### User Story 3 - Límite del asistente sin reescritura documentado y congelado (Priority: P3)

La persona que usa el asistente sin reescritura de salidas encuentra documentado que el hook no reescribe por límite del runtime (no es un bug pendiente) y que su ahorro llega por el paquete de contexto y la búsqueda de código que comprime en origen. El comportamiento de no emitir se mantiene intacto.

**Why this priority**: Es la brecha 3 del informe ("nada que corregir"). Sin esta historia, el límite se re-reporta como defecto en cada auditoría futura.

**Independent Test**: Se puede probar sin tocar código: el diagnóstico y la documentación describen el límite del runtime, y una prueba con salida grande confirma que el hook no emite nada y sale con éxito.

**Acceptance Scenarios**:

1. **Given** la documentación de compresión, **When** la persona la lee para ese asistente, **Then** encuentra el límite del runtime declarado con las dos vías de ahorro que sí aplican.
2. **Given** una salida grande y compresible, **When** el hook de ese asistente la procesa, **Then** no emite reescritura y termina con éxito (la salida original se conserva).

---

### User Story 4 - Ayuda organizada por flujos con el ahorro visible (Priority: P2)

La persona ejecuta la ayuda en línea y encuentra los comandos agrupados por flujo de trabajo (memoria diaria, contexto y ahorro, sesiones, revisión, instalación, documentos, mantenimiento, sistema), cada uno con una línea que dice qué hace, más ejemplos de uso. Los comandos de paquete y ahorro, hoy invisibles en la ayuda, aparecen en su grupo.

**Why this priority**: La auditoría reveló que los comandos que materializan la promesa de ahorro ni siquiera aparecen en la ayuda. Sin esta historia, FR-001 promete "textos de ayuda en línea" que nadie descubre.

**Independent Test**: Se puede probar solo con la ayuda: cualquier subcomando del despachador aparece mencionado, y una persona nueva localiza el comando de ahorro sin conocer su nombre de antemano.

**Acceptance Scenarios**:

1. **Given** la ayuda en línea, **When** la persona busca cómo ahorrar contexto, **Then** encuentra los comandos de paquete y ahorro con la distinción entre ahorro por emisión y ahorro por compresor.
2. **Given** cualquier subcomando aceptado por el despachador, **When** la persona lee la ayuda completa, **Then** el subcomando está mencionado (ningún comando invisible).

---

### Edge Cases

- ¿Qué pasa cuando las cifras medidas cambian entre versiones? La documentación indica cómo reproducir la medición en lugar de congelar números que se vuelven falsos.
- ¿Cómo se evita que el diagnóstico mienta en harnesses futuros? Cada estado de hook del diagnóstico debe corresponder a un comportamiento verificado en vivo, no a documentación del proveedor.
- ¿Qué pasa si el runtime del editor empieza a aplicar (o deja de aceptar) la sustitución? El estado best-effort sigue siendo veraz en ambos casos; solo cambia si se verifica un comportamiento distinto.
- ¿Qué pasa si alguien pide compresión agresiva del código? Queda fuera de alcance: la preservación de código y lecturas exactas no se relaja en esta feature.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La documentación de ahorro DEBE describir las tres vías por separado (contexto emitido, paquete de contexto bajo presupuesto, hook de salidas) con el orden de magnitud medido de cada una (cifra con fecha de medición) y el comando para reproducir la medición, en la guía de instalación, el manual y los textos de ayuda en línea. La TUI queda fuera de alcance.
- **FR-002**: La documentación DEBE declarar los casos en que el hook deja la salida intacta (contenido bajo el umbral mínimo, código fuente, prosa en modo salida, lecturas exactas y diagnósticos) y el motivo (preservar lo editable y lo literal).
- **FR-003**: La ayuda del comando de ahorro por compresor DEBE distinguir el ahorro por compresor del ahorro por emisión de contexto, sin mezclar ambas cifras.
- **FR-004**: El diagnóstico DEBE reportar el hook del editor con plugin como emisión best-effort (activo, con advertencia de que la sustitución depende del runtime) y NO como fallo o estado sin verificar.
- **FR-005**: La documentación DEBE declarar el límite del asistente sin reescritura (el runtime rechaza la reescritura, no es un bug) junto con las vías de ahorro que sí le aplican (paquete de contexto, búsqueda de código en origen).
- **FR-006**: El comportamiento del hook para el asistente sin reescritura NO DEBE cambiar: ante cualquier salida, no emite reescritura y termina con éxito.
- **FR-007**: Ningún cambio de esta feature DEBE relajar las preservaciones existentes (código, URLs, rutas, errores, lecturas exactas) ni el umbral mínimo del hook.
- **FR-008**: La ayuda en línea DEBE agrupar los comandos por flujo de trabajo con una línea descriptiva por comando, en lugar de una lista plana.
- **FR-009**: Todos los subcomandos aceptados por el despachador DEBEN estar mencionados en la ayuda, incluidos los hoy invisibles (paquete, semilla, sincronización de ADR).
- **FR-010**: La ayuda en línea DEBE seguir el patrón de bloques del referente (encabezado `COMMANDS` con grupos por flujo, bloque `EJEMPLOS COMUNES`, bloque `OPCIONES`), y el reporte de `usage` DEBE organizarse en bloques `RESUMEN` / `POR OPERACIÓN` / `POR CANAL` con cierre de totales.

### Key Entities

- **Vía de ahorro**: cada uno de los tres mecanismos (contexto emitido, paquete de contexto, hook de salidas) con su métrica y su método de reproducción.
- **Estado de hook por harness**: el estado reportado por el diagnóstico para cada asistente, verificado contra comportamiento en vivo.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Una persona nueva describe correctamente el ahorro esperado de cada vía y los casos intactos del hook tras leer solo la documentación (3 de 3 preguntas de sondeo).
- **SC-002**: Con el plugin instalado, el diagnóstico muestra cero problemas atribuidos al hook de salidas de ese editor.
- **SC-003**: Una salida grande y repetitiva procesada por el hook del asistente de terminal y la del editor con plugin produce reescritura con referencia recuperable cuyo contenido original se recupera íntegro.
- **SC-004**: El hook del asistente sin reescritura no emite nada y termina con éxito ante la misma salida, y su límite está documentado.
- **SC-005**: Ninguna prueba existente de preservación (código, lecturas exactas, diagnósticos) se modifica ni se relaja para cumplir esta especificación.
- **SC-006**: El 100% de los subcomandos aceptados por el despachador aparece mencionado en la ayuda (verificado por contrato automatizado).

## Assumptions

- Las cifras concretas de ahorro se documentan como órdenes de magnitud con fecha de medición y comando de reproducción, no como garantías contractuales.
- El alcance es documentación + textos de ayuda y diagnóstico en línea (incluye reagrupar la ayuda, US4); no incluye cambios en el motor de compresión ni en los umbrales del hook.
- El comportamiento del runtime del asistente sin reescritura se toma como dado externo; si el proveedor lo cambia, se abrirá una feature nueva.
- La verificación en vivo se hace con el binario instalado y salidas sintéticas grandes y repetitivas (p. ej. listados de 60 elementos), siguiendo el método de la auditoría [378].
