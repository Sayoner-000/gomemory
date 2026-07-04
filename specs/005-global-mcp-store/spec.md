# Feature Specification: Registro global de gomemory (sin instalación por proyecto)

**Feature Branch**: `005-global-mcp-store`

**Created**: 2026-07-03

**Status**: Draft

**Input**: User description: "Plan: gomemory sin instalación por proyecto (store global + registro MCP global) — hoy usar gomemory exige `mem install <dir>` por repo (copia binario, crea `.memory/mem.db`, escribe configs MCP por agente, inyecta protocolo en AGENTS.md/CLAUDE.md), lo que genera fricción y ya causó un bug real de marcador de protocolo huérfano entre versiones. Se compara contra `codebase-memory-mcp`, disponible desde el arranque de sesión sin instalación por proyecto porque se registra una sola vez a nivel de usuario. Se decide: la base de datos deja de vivir en `<repo>/.memory/mem.db` y pasa a un store global del usuario (seguro porque `.memory/` ya está gitignoreado hoy); la identidad de proyecto deja de ser el nombre de carpeta y pasa a ser la ruta absoluta del git root; el registro del MCP server pasa a ser global (una vez por máquina) donde el agente lo soporte, con fallback por-proyecto donde no. Se detectó además una colisión de nombre real en el registro global existente de Claude Code que debe resolverse como parte del rollout."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Usar gomemory en un proyecto nuevo sin instalar nada (Priority: P1)

Como usuario que abre un proyecto por primera vez (clonado o recién creado), quiero que gomemory esté disponible de inmediato para guardar y consultar memoria, sin ejecutar ningún comando de instalación en ese repo, igual que ya ocurre con otras herramientas MCP registradas a nivel global.

**Why this priority**: Es el problema central que motiva el cambio — la fricción de "instalar por repo" es lo que hace que gomemory se sienta más pesado que alternativas que ya funcionan sin instalación.

**Independent Test**: Se puede probar por completo abriendo un repositorio nunca antes usado con gomemory (sin `.memory/` previo) e invocando una herramienta de memoria (guardar o buscar); el valor entregado es memoria funcional desde el primer uso.

**Acceptance Scenarios**:

1. **Given** un repositorio nuevo sin ningún rastro previo de gomemory, **When** el agente invoca `save_memory` o `search_memories` por primera vez, **Then** la operación se completa con éxito, sin pedir ejecutar un comando de instalación o inicialización previo.
2. **Given** ese mismo repositorio ya en uso, **When** se guarda una memoria y luego se busca en una sesión posterior, **Then** la memoria guardada aparece en los resultados, confirmando que el proyecto quedó identificado de forma consistente entre sesiones.

---

### User Story 2 - Conservar la memoria ya guardada en proyectos instalados a la manera antigua (Priority: P2)

Como usuario que ya tiene proyectos con memoria guardada bajo el modelo actual (instalación por repo), quiero que esa memoria se conserve íntegra al pasar al nuevo modelo, para no perder decisiones, bugfixes o aprendizajes ya documentados.

**Why this priority**: Sin esto, el cambio de arquitectura implicaría pérdida de datos para todo usuario existente — inaceptable aunque el modelo nuevo sea mejor.

**Independent Test**: Se puede probar tomando un proyecto con memorias existentes, activando el nuevo modelo, y comparando el conteo y contenido de memorias antes/después.

**Acceptance Scenarios**:

1. **Given** un proyecto con N memorias guardadas bajo el modelo anterior, **When** el proyecto se usa por primera vez bajo el nuevo modelo, **Then** las N memorias siguen accesibles vía búsqueda y listado, sin duplicados ni pérdidas.
2. **Given** un proyecto donde por algún motivo coexisten datos en la ubicación antigua y en la nueva, **When** se detecta esa coexistencia, **Then** el sistema no sobreescribe silenciosamente ninguna de las dos fuentes y lo señala de forma visible al usuario.

---

### User Story 3 - Mantener la memoria aislada entre proyectos distintos (Priority: P2)

Como usuario que trabaja en varios proyectos, incluyendo algunos que comparten nombre de carpeta, quiero que la memoria de cada proyecto permanezca separada de la de los demás, para que una decisión o bugfix de un proyecto nunca aparezca mezclado con el de otro.

**Why this priority**: El modelo global multiplexa muchos proyectos en un solo servidor — si la identidad de proyecto es ambigua, esta característica introduce un riesgo de contaminación cruzada que no existía en el modelo por-archivo-separado actual.

**Independent Test**: Se puede probar creando dos repositorios distintos con el mismo nombre de carpeta en rutas diferentes, guardando una memoria distinta en cada uno, y verificando que cada uno solo ve la suya.

**Acceptance Scenarios**:

1. **Given** dos repositorios en rutas distintas que comparten el mismo nombre de carpeta, **When** se guarda una memoria en cada uno, **Then** buscar memorias desde cada repositorio solo devuelve las memorias guardadas en ese repositorio.

---

### User Story 4 - Registrar el servidor MCP una sola vez por máquina (Priority: P3)

Como usuario de un agente compatible con registro de MCP a nivel de usuario (ej. Claude Code), quiero registrar gomemory una sola vez y que funcione automáticamente en todos mis proyectos, sin repetir el registro por cada repo nuevo.

**Why this priority**: Es la contraparte de la fricción de instalación en el lado de configuración del agente; sin esto, aunque el store de datos sea global, seguiría existiendo un paso manual por proyecto.

**Independent Test**: Se puede probar registrando gomemory una vez en un agente compatible y luego abriendo dos proyectos distintos nunca antes usados, confirmando que ambos ven las herramientas de gomemory sin registro adicional.

**Acceptance Scenarios**:

1. **Given** gomemory registrado una vez a nivel de usuario en un agente compatible, **When** se abre un proyecto distinto nunca antes usado con gomemory, **Then** las herramientas de gomemory están disponibles sin ningún paso de registro adicional en ese proyecto.
2. **Given** un agente que no soporta registro a nivel de usuario, **When** se quiere usar gomemory en un proyecto con ese agente, **Then** sigue existiendo una forma de registrarlo por proyecto (comportamiento actual, sin regresión).

---

### Edge Cases

- ¿Qué pasa cuando el mismo repositorio existe clonado en dos rutas distintas de la misma máquina? Cada ubicación se trata como proyecto independiente (memorias no compartidas entre clones) — comportamiento esperado y documentado, no un defecto.
- ¿Qué pasa si el proyecto no tiene `.git` inicializado? El directorio de trabajo actual se usa como identidad del proyecto; si ese directorio se mueve o renombra, se trata como un proyecto nuevo.
- ¿Qué pasa si ya existe, a nivel de usuario, un registro con el mismo nombre perteneciente a otra herramienta (colisión de nombre)? El sistema debe permitir detectar y resolver esa colisión de forma explícita antes de activar el registro global de gomemory — no debe sobrescribir silenciosamente el registro de otra herramienta sin que el usuario lo confirme.
- ¿Qué pasa si dos sesiones de agentes distintos (o el mismo agente en dos ventanas) acceden al mismo proyecto simultáneamente? Debe seguir funcionando igual que hoy dentro de un mismo repo (ya soportado).
- ¿Qué pasa con proyectos que ya tienen archivos de instalación previos (binario copiado, configs MCP por proyecto, bloques de protocolo en AGENTS.md/CLAUDE.md)? Deben poder seguir funcionando durante la transición, y el usuario debe tener una forma clara de limpiar esos artefactos una vez migrado.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE identificar el proyecto activo automáticamente a partir de la ubicación del repositorio, sin requerir un paso de instalación o inicialización previo en ese repositorio.
- **FR-002**: El sistema DEBE mantener el historial de memoria completamente aislado por proyecto: ninguna memoria guardada en un proyecto debe ser visible ni recuperable desde otro proyecto.
- **FR-003**: El sistema DEBE permitir que un proyecto nunca antes usado guarde y consulte memoria en su primer uso, sin comando de inicialización manual previo.
- **FR-004**: El sistema DEBE preservar sin pérdida ni duplicación las memorias guardadas bajo el modelo de instalación por proyecto anterior, al transicionar un proyecto existente al nuevo modelo.
- **FR-005**: El sistema NO DEBE requerir copiar archivos (binario, configuración de MCP, instrucciones de protocolo) dentro del árbol de trabajo de un proyecto para que la memoria funcione en él.
- **FR-006**: El sistema DEBE permitir registrar el servidor MCP una sola vez a nivel de usuario/máquina, para los agentes que soporten ese modo de registro, de forma que aplique automáticamente a cualquier proyecto.
- **FR-007**: El sistema DEBE seguir ofreciendo una forma de registro por proyecto como alternativa, para agentes que no soporten registro a nivel de usuario.
- **FR-008**: El sistema DEBE detectar cuando el registro global de gomemory entraría en conflicto con un registro existente del mismo nombre perteneciente a otra herramienta, y requerir confirmación explícita antes de sobrescribirlo.
- **FR-009**: El sistema DEBE ofrecer una forma de identificar y limpiar, de forma explícita, los artefactos de instalación dejados por el modelo anterior en un proyecto (binario copiado, configuración MCP local, bloques de protocolo inyectados) una vez migrado al nuevo modelo.

### Key Entities

- **Proyecto**: unidad de aislamiento de memoria; su identidad se basa en la ubicación real del repositorio (no en el nombre de su carpeta), de forma que dos proyectos con el mismo nombre nunca se confunden entre sí.
- **Memoria**: registro persistente (decisión, bugfix, patrón, preferencia, aprendizaje, etc.) asociado siempre a exactamente un proyecto.
- **Registro de servidor MCP**: configuración, específica de cada agente, que determina si gomemory está disponible de forma global (todos los proyectos, un solo registro) o local (un registro por proyecto).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un usuario puede empezar a guardar y buscar memoria en un repositorio nunca antes usado sin ejecutar ningún comando de instalación, en el primer intento.
- **SC-002**: El 100% de las memorias existentes en proyectos previamente instalados bajo el modelo anterior siguen accesibles después de la transición, verificado por conteo antes/después.
- **SC-003**: Usar gomemory en un proyecto no introduce ningún archivo nuevo visible en el estado de control de versiones de ese proyecto (excepto los artefactos legados que ya existían antes de este cambio, hasta que se limpien explícitamente).
- **SC-004**: Dos proyectos distintos que comparten el mismo nombre de carpeta nunca comparten memorias entre sí, verificado con al menos un caso de prueba explícito.
- **SC-005**: Un agente compatible con registro a nivel de usuario, una vez registrado gomemory globalmente, tiene acceso a las herramientas de gomemory en cualquier proyecto nuevo sin pasos adicionales.

## Assumptions

- Se opera en una máquina de un solo usuario del sistema operativo; el aislamiento entre distintas cuentas de usuario del mismo equipo no es un requisito de esta feature.
- Los proyectos suelen usar git; cuando no lo usan, el directorio de trabajo actual se acepta como identidad del proyecto, asumiendo que el usuario entiende que mover ese directorio equivale a crear un proyecto distinto.
- Al menos uno de los agentes soportados (Claude Code) ya ofrece un mecanismo de registro de servidores MCP a nivel de usuario, verificado en el entorno actual de desarrollo.
- Si al resolver una colisión de nombre en el registro global se retira el registro de otra herramienta, se asume que esa consecuencia se comunica al usuario como parte del proceso — no se asume responsabilidad por el impacto en esa otra herramienta.
- Los agentes que no soporten registro a nivel de usuario seguirán requiriendo un paso de configuración por proyecto; esto se documenta como limitación de esos agentes, no como incumplimiento de esta feature.
