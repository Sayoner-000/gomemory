# Feature Specification: Mitigación de riesgos operativos de gomemory

**Feature Branch**: `009-mitigacion-riesgos`

**Created**: 2026-07-18

**Status**: Draft

**Input**: User description: "Mitigación de riesgos de gomemory (proyecto de 1 mes / 1 autor). Cuatro mejoras independientes: (1) búsqueda FTS5 con fallback a LIKE, (2) backup automático local al fin de sesión, (3) redacción de secretos en dos capas + hardening de permisos de archivo, (4) convención de compatibilidad documentada para el formato de export y las migraciones de esquema."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Recuperar memorias tras pérdida de datos (Priority: P1)

Como autor único de gomemory, cuando pierdo o corrompo el archivo de base de datos de un proyecto (fallo de disco, `rm` accidental, reinstalación de la máquina), quiero poder recuperar las memorias guardadas hasta el snapshot automático más reciente sin haber tenido que acordarme de exportar manualmente antes del incidente.

**Why this priority**: Es el único de los cuatro riesgos con una consecuencia irreversible (pérdida permanente de memorias) y sin ninguna mitigación existente hoy. El resto de los riesgos degradan calidad o exponen datos, pero no destruyen información.

**Independent Test**: Cerrar una sesión de trabajo sobre un proyecto con memorias guardadas, verificar que aparece un snapshot nuevo en el directorio de backups, borrar (o renombrar) la base de datos activa, y confirmar que el snapshot más reciente permite restaurar el mismo conjunto de memorias.

**Acceptance Scenarios**:

1. **Given** un proyecto con memorias guardadas y una sesión activa, **When** la sesión termina, **Then** se genera un snapshot nuevo con marca de tiempo en un directorio de backups dedicado, sin que el usuario tenga que invocar nada manualmente.
2. **Given** que ya existen N snapshots del mismo proyecto (N = límite configurado), **When** se genera un snapshot adicional, **Then** el snapshot más antiguo se elimina automáticamente y el total nunca supera el límite.
3. **Given** un snapshot existente, **When** se restaura sobre un proyecto vacío, **Then** todas las memorias y relaciones contenidas en el snapshot quedan disponibles de nuevo, sin duplicados si se restaura dos veces.
4. **Given** que el proceso de generación de snapshot falla por cualquier motivo (disco lleno, permisos, etc.), **When** la sesión termina, **Then** el cierre de sesión se completa igual, sin bloquear ni mostrar error al usuario.

---

### User Story 2 - Evitar que un secreto pegado quede en texto plano (Priority: P2)

Como autor único de gomemory, cuando pego contenido en una memoria que incluye un token o credencial que olvidé envolver en `<private>`, quiero que patrones de secretos reconocibles (claves de AWS, tokens de GitHub, claves de API de proveedores de IA, tokens de Slack, JWT, bloques de clave privada PEM) se redacten igual, y que el archivo de base de datos no sea legible por otros usuarios del mismo sistema operativo.

**Why this priority**: Expone datos sensibles de forma silenciosa — el usuario cree que está protegido por la convención `<private>` pero solo lo está para lo que recuerda envolver. Es menos grave que la pérdida irreversible de datos (User Story 1) porque el contenido sigue en manos del propio usuario, pero la exposición ya ocurrió en cuanto se persiste.

**Independent Test**: Guardar una memoria cuyo contenido incluya un token de prueba con formato reconocible (ej. una clave de AWS de ejemplo) sin envolverlo en `<private>`, y verificar directamente en el archivo de base de datos que el token no aparece en texto plano. Verificar también que un archivo de base de datos recién creado no es legible por otros usuarios del sistema.

**Acceptance Scenarios**:

1. **Given** una memoria cuyo contenido incluye un patrón de secreto reconocido (AWS, GitHub, proveedor de IA, Slack, JWT, o clave privada PEM) fuera de cualquier bloque `<private>`, **When** la memoria se guarda, **Then** el patrón detectado se reemplaza por un marcador de redacción antes de persistirse.
2. **Given** contenido que no coincide con ningún patrón conocido de secreto, **When** la memoria se guarda, **Then** el contenido se persiste sin modificaciones (no hay falsos positivos que degraden memorias legítimas).
3. **Given** un archivo de base de datos recién creado para un proyecto nuevo, **When** se inspeccionan sus permisos a nivel de sistema operativo, **Then** solo el usuario propietario puede leer o escribir el archivo y el directorio que lo contiene.
4. **Given** contenido envuelto en `<private>...</private>`, **When** la memoria se guarda, **Then** ese contenido se sigue eliminando por completo como ya ocurre hoy (esta mejora no reemplaza la redacción existente, la complementa).

---

### User Story 3 - Encontrar la memoria correcta cuando el volumen crece (Priority: P3)

Como autor único de gomemory, cuando busco una memoria entre muchas guardadas en un proyecto activo, quiero que los resultados más relevantes aparezcan primero (no solo los más recientes que contienen el texto), y que la búsqueda siga siendo rápida a medida que el número de memorias del proyecto crece.

**Why this priority**: Hoy el volumen máximo observado (135 memorias en el proyecto más activo) no genera un problema de rendimiento perceptible ni de relevancia grave — es una mejora de calidad progresiva, no una corrección de algo roto. Se prioriza por debajo de los riesgos que ya causan daño (pérdida de datos, exposición de secretos).

**Independent Test**: Sobre un proyecto con varias decenas de memorias, ejecutar una búsqueda con un término que aparece tanto en títulos como en contenido de múltiples memorias, y confirmar que el orden de resultados refleja relevancia real (coincidencias más fuertes primero) y no solo si el término está en el título vs. el contenido.

**Acceptance Scenarios**:

1. **Given** un proyecto con memorias cuyo título y contenido contienen distintas variantes de un término de búsqueda, **When** se ejecuta una búsqueda con ese término, **Then** los resultados se ordenan por relevancia real del texto, no solo por la categoría título/contenido/ninguno usada hoy.
2. **Given** un entorno donde el motor de búsqueda de texto completo no está disponible (por restricciones de compilación), **When** se ejecuta una búsqueda, **Then** el sistema sigue devolviendo resultados correctos usando el mecanismo de coincidencia simple existente, sin errores visibles al usuario.
3. **Given** que se crea, actualiza o elimina una memoria, **When** la operación se completa, **Then** el índice de búsqueda queda sincronizado con el contenido vigente (sin resultados obsoletos ni faltantes).

---

### User Story 4 - No perder compatibilidad entre versiones futuras (Priority: P4)

Como autor único de gomemory, cuando retomo el proyecto después de un tiempo o pido a un agente que modifique el formato de export o el esquema de la base de datos, quiero que exista una convención documentada y visible en el propio código que impida romper la compatibilidad con backups o exports anteriores sin darme cuenta.

**Why this priority**: Es una salvaguarda de proceso, no una corrección de un defecto activo — hasta ahora todos los cambios de esquema han sido aditivos y compatibles. El riesgo es prospectivo (evitar una futura regresión), por lo que tiene la prioridad más baja de las cuatro.

**Independent Test**: Revisar el código fuente en los puntos donde se define el esquema de la base de datos y el formato de export, y confirmar que la convención de compatibilidad queda documentada explícitamente ahí (no solo en un documento externo que se puede perder de contexto).

**Acceptance Scenarios**:

1. **Given** el punto del código donde se aplican los cambios de esquema de la base de datos, **When** se lee ese código, **Then** queda documentada la regla de que los cambios deben ser aditivos (nuevas columnas/tablas) y no destructivos.
2. **Given** el punto del código donde se define la versión del formato de export, **When** se lee ese código, **Then** queda documentada la regla de que cualquier cambio de campo requiere subir la versión y agregar una función de migración explícita entre versiones.

---

### Edge Cases

- ¿Qué pasa si el directorio de backups no existe todavía la primera vez que se cierra una sesión? Debe crearse automáticamente.
- ¿Qué pasa si dos patrones de secretos se superponen en el mismo fragmento de texto (ej. un JWT dentro de un bloque más grande)? Debe redactarse sin dejar fragmentos parciales del secreto original.
- ¿Qué pasa si el usuario restaura el mismo snapshot de backup dos veces sobre el mismo proyecto? No debe generar memorias duplicadas.
- ¿Qué pasa si una memoria ya existente (guardada antes de esta mejora) contiene un secreto sin redactar? Queda fuera de alcance reprocesar memorias históricas; la mitigación aplica solo hacia adelante.
- ¿Qué pasa si el sistema operativo no soporta el modelo de permisos Unix (ej. Windows)? El hardening de permisos se aplica en la medida que la plataforma lo soporte, sin fallar la operación si no es posible.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE generar automáticamente un snapshot exportable de las memorias y relaciones de un proyecto al finalizar cada sesión de trabajo, sin requerir una acción manual del usuario.
- **FR-002**: El sistema DEBE conservar únicamente los snapshots automáticos más recientes hasta un límite configurado, descartando los más antiguos al superarlo.
- **FR-003**: El sistema DEBE permitir restaurar cualquier snapshot automático generado, usando el mecanismo de importación ya existente, sin generar duplicados ante restauraciones repetidas.
- **FR-004**: La generación de un snapshot automático NO DEBE interrumpir ni hacer fallar el cierre de sesión si el snapshot no puede completarse.
- **FR-005**: El sistema DEBE detectar y redactar automáticamente patrones de secretos reconocibles (al menos: claves de AWS, tokens de GitHub, claves de API de proveedores de IA, tokens de Slack, JWT, bloques de clave privada PEM) en el título y contenido de una memoria antes de persistirla, además de la redacción existente de bloques `<private>`.
- **FR-006**: El sistema DEBE aplicar la redacción de patrones de secretos y de bloques `<private>` en el momento de guardar la memoria, de forma que el contenido sin redactar nunca llegue a escribirse en el almacenamiento persistente.
- **FR-007**: El sistema DEBE restringir los permisos del archivo de base de datos y de sus directorios contenedores para que solo el usuario propietario del sistema operativo pueda leerlos o escribirlos, en las plataformas donde ese modelo de permisos existe.
- **FR-008**: El sistema DEBE ordenar los resultados de búsqueda de memorias por relevancia real del texto respecto al término buscado, no únicamente por si la coincidencia ocurrió en el título o en el contenido.
- **FR-009**: El sistema DEBE seguir devolviendo resultados de búsqueda correctos mediante el mecanismo de coincidencia simple existente cuando el motor de búsqueda de texto completo no esté disponible en el entorno de ejecución.
- **FR-010**: El índice usado para ordenar por relevancia DEBE mantenerse sincronizado con las memorias vigentes ante creaciones, actualizaciones y eliminaciones.
- **FR-011**: El código fuente DEBE documentar explícitamente, en el punto donde se define el esquema de la base de datos, la convención de que los cambios de esquema son aditivos y no destructivos.
- **FR-012**: El código fuente DEBE documentar explícitamente, en el punto donde se define la versión del formato de export, la regla de que un cambio de campo requiere subir la versión y agregar una función de migración explícita.

### Key Entities

- **Snapshot de backup**: copia exportada en un momento dado de las memorias y relaciones de un proyecto; tiene una marca de tiempo y pertenece a un proyecto específico; es efímero según el límite de retención configurado.
- **Patrón de secreto**: una forma de texto reconocible que corresponde a un tipo conocido de credencial (AWS, GitHub, proveedor de IA, Slack, JWT, clave privada PEM); al detectarse en una memoria, se reemplaza por un marcador de redacción antes de persistir.
- **Índice de relevancia**: estructura auxiliar derivada del título y contenido de las memorias de un proyecto, usada para ordenar resultados de búsqueda por relevancia en lugar de solo por categoría de coincidencia y recencia.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Ante la pérdida completa del archivo de base de datos activo de un proyecto, el usuario puede recuperar el 100% de las memorias guardadas hasta el snapshot automático más reciente, sin haber tenido que exportar manualmente antes del incidente.
- **SC-002**: Ningún secreto con un patrón reconocido (de los seis tipos cubiertos) que se pegue en una memoria sin envolver en `<private>` queda legible en texto plano en el almacenamiento persistente.
- **SC-003**: Un archivo de base de datos nuevo no es legible por ningún otro usuario del sistema operativo distinto del propietario, verificable inmediatamente después de su creación.
- **SC-004**: En un proyecto con decenas de memorias, una búsqueda por un término presente en varias memorias devuelve la memoria más relevante entre los primeros resultados, no solo la más reciente.
- **SC-005**: El tiempo de respuesta de una búsqueda no se degrada de forma perceptible para el usuario a medida que el número de memorias del proyecto crece dentro de los rangos de uso actuales.

## Assumptions

- El "usuario" de esta especificación es el autor único de gomemory, operando en su propia máquina local; no hay múltiples usuarios finales ni escenarios multi-tenant.
- Los snapshots automáticos cubren memorias y relaciones (lo que ya cubre el mecanismo de exportación existente); no se espera que cubran sesiones ni el grafo de código, que se consideran regenerables o de bajo valor para recuperación.
- La sincronización entre máquinas (llevar los snapshots a otra computadora) se resuelve con herramientas externas ya elegidas por el usuario (servicio de sincronización de archivos, repositorio privado, etc.), no es responsabilidad de esta funcionalidad generar o gestionar esa sincronización.
- La lista de patrones de secretos reconocidos es fija y conocida de antemano (no hay aprendizaje ni detección genérica basada en entropía); patrones no listados no se detectan.
- El hardening de permisos de archivo asume un sistema operativo tipo Unix (macOS/Linux) como entorno principal; en plataformas sin ese modelo de permisos, la protección no aplica pero tampoco debe romper el funcionamiento.
- No se espera que memorias guardadas antes de esta funcionalidad sean reprocesadas retroactivamente para redactar secretos o reindexar búsqueda; ambas mejoras aplican solo a operaciones futuras.
- El cifrado en reposo de la base de datos completa, la sincronización nativa entre máquinas, y la búsqueda semántica basada en embeddings quedan fuera de alcance de esta especificación.
