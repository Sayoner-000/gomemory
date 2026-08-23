# Contrato: la matriz de canales y su verificación

Este contrato define qué debe cumplir la declaración única y qué comprueba la batería. Es el
mecanismo que cierra la fuente de defectos: una celda sin declarar deja de ser un descubrimiento
futuro y pasa a ser un fallo inmediato.

## C1 — Completitud de la celda

Toda celda declara `Path` o `NotApplicableReason`, nunca ambos ni ninguno.

- **Falla si**: existe una celda sin ruta y sin motivo.
- **Mensaje**: nombra agente, tipo de canal y ámbito.
- **Cubre**: FR-004, FR-005.

## C2 — Integridad referencial

Todo `Agent` de la matriz existe en `KnownAgents`, y todo agente de `KnownAgents` que declare un
nivel en un ámbito tiene celda para ese canal en ese ámbito, o motivo declarado.

- **Falla si**: una celda referencia un agente inexistente, o un agente declara una capacidad
  sin celda ni motivo.
- **Cubre**: FR-006, FR-019, SC-006.

## C3 — Simetría entre instalar y desinstalar

El conjunto de celdas de ámbito de proyecto con `Managed` que escribe la instalación es igual al
conjunto que retira la desinstalación.

- **Falla si**: existe una celda escrita por una y no retirada por la otra.
- **Cubre**: FR-008, FR-012, SC-003.

## C4 — Contención del ámbito

Ninguna actividad cuyo alcance declarado sea `ScopeProject` referencia celdas `ScopeUser`.

- **Falla si**: la derivación de una actividad de proyecto incluye una celda de usuario.
- **Cubre**: FR-013, FR-015.

## C5 — El entorno de la persona no se modifica

Ejercer cualquier actividad de alcance de proyecto sobre un directorio temporal deja el
directorio de la persona sin cambios, comprobado por inventario antes y después.

- **Falla si**: aparece, desaparece o cambia cualquier archivo del entorno de la persona.
- **Cubre**: FR-016, SC-004.
- **Nota**: este contrato es el que habría detectado el defecto que eliminó un complemento real
  durante el desarrollo de esta misma feature.

## C6 — Rutas relativas

Ninguna celda declara una ruta absoluta ni un segmento que escape del directorio destino.

- **Falla si**: un `Path` comienza por separador, contiene `..`, o incluye el directorio de la
  persona.
- **Cubre**: INV-2, principio I de la constitución.

## C7 — Acuerdo de las tablas no migradas

Cada tabla que todavía no se deriva de la matriz debe coincidir exactamente con la proyección
que la matriz produce para su ámbito y tipo de canal.

- **Falla si**: una tabla declara un artefacto que la matriz no tiene, o le falta uno que la
  matriz sí declara.
- **Cubre**: FR-003 en su forma verificada, R3.
- **Nota**: este contrato es lo que permite consolidar sin reescribir todos los adaptadores en
  un solo cambio. Una tabla que coincide y falla al dejar de coincidir ya no es una isla.

## C8 — Selección de agentes respetada

Una actividad que recibe una selección de agentes produce artefactos únicamente de esos agentes.

- **Falla si**: tras solicitar un agente aparece un artefacto de otro, incluido su directorio de
  configuración.
- **Cubre**: FR-017, FR-018, SC-005.
