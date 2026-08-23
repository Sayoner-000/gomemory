# Fase 0 — Investigación

Todas las incógnitas se resolvieron leyendo el código y midiendo contra el binario publicado.
No quedan marcadores `NEEDS CLARIFICATION`.

## R1 — ¿Cuántas fuentes de verdad hay realmente?

**Decisión**: catorce declaraciones independientes de artefactos por agente. Tres consultan el
registro de capacidades, y una de esas tres lo anula con un filtro fijo a un agente.

**Verificación**: conteo sobre las declaraciones de paquete del repositorio, excluyendo las
listas de operaciones MCP (que no son por agente) y el propio registro.

**Consecuencia para el diseño**: la matriz no puede ser «una tabla más». Si no absorbe o ata a
las catorce, será la número quince.

---

## R2 — ¿Se puede extender el vocabulario existente o hace falta uno nuevo?

**Decisión**: extender. El dominio ya declara `ChannelArm`, `ChannelKind` (seis tipos),
`AgentScope` (dos ámbitos) y `ChannelState`. La matriz añade la celda que une agente, tipo y
ámbito con el artefacto correspondiente.

**Alternativa descartada**: un modelo paralelo de «canales instalables». Habría duplicado el
vocabulario del diagnóstico y creado justamente el problema que la feature ataca.

---

## R3 — ¿Migrar los catorce consumidores o atarlos?

**Decisión**: migrar dos, atar el resto con pruebas de acuerdo.

**Rationale**: los dos consumidores que se migran son los que produjeron defectos verificados,
la desinstalación y el registro de ámbito global. Los demás funcionan hoy; su riesgo no es
comportarse mal, sino separarse de la fuente en el futuro. Una prueba de acuerdo cubre ese
riesgo exacto con una fracción del cambio.

**Alternativa descartada**: migrar los catorce en un solo cambio. Superficie de error grande,
contra el principio de impacto mínimo, y del tipo de refactor que introduce los defectos que
esta feature previene.

**Límite declarado**: una tabla atada por prueba sigue siendo una copia. La garantía es que no
puede divergir sin que la batería falle, no que no exista.

---

## R4 — ¿Cómo se verifica que una celda no quedó sin declarar?

**Decisión**: un contrato en `tests/contract/` recorre la matriz y exige, por cada celda, o bien
un artefacto declarado, o bien un motivo declarado. Una celda sin ninguno de los dos hace fallar
la batería nombrándola.

**Rationale**: la ausencia silenciosa es el estado que produjo los cuatro defectos. Convertirla
en un fallo ruidoso es el objetivo de la feature, y una prueba es el mecanismo más simple que lo
consigue sin maquinaria adicional.

---

## R5 — ¿Cómo se garantiza que una actividad de proyecto no toque el ámbito de la persona?

**Decisión**: cada actividad declara su alcance; el contrato la ejerce sobre un directorio
temporal con el entorno de la persona redirigido y falla si ese entorno resulta modificado.

**Alternativa descartada**: impedirlo por tipos, con operaciones de archivo distintas por
ámbito. Obligaría a envolver toda la escritura del proyecto para una garantía que la prueba da
igual de bien.

**Hallazgo que lo motiva**: durante esta misma sesión, añadir a la desinstalación una operación
sobre una ruta derivada del directorio de la persona convirtió pruebas inofensivas en
destructivas, y llegó a eliminar el complemento instalado en la máquina real.

---

## R6 — ¿Cuántas pruebas existentes exponen el entorno de la persona?

**Decisión**: seis funciones en dos archivos.

**Verificación**: de los seis archivos que ejercen actividades de ciclo de vida, tres ya aíslan
el entorno. De los tres restantes, uno opera con rutas explícitas y no se ve afectado. Quedan
cinco funciones en las pruebas de integración de desinstalación y una en las de contrato de
mantenimiento.

**Consecuencia para el orden de tareas**: el aislamiento va **antes** de migrar la
desinstalación. Hacerlo al revés reexpone el entorno de quien ejecute la batería durante el
propio desarrollo de la feature.

**Nota de gobernanza**: modificar pruebas existentes requiere autorización explícita según el
principio III de la constitución. Está concedida y registrada en `plan.md`.

---

## R7 — ¿Qué esquema de configuración usa cada agente?

**Decisión**: el esquema es un dato de la celda, no una constante compartida.

**Verificación**: los agentes no coinciden. Uno registra sus servidores bajo una clave y el
resto bajo otra, con formas distintas para el comando. Asumir un esquema único fue la causa de
que una entrada sobreviviera a toda desinstalación.

---

## R8 — ¿Qué pasa con los agentes que solo reciben el registro del servidor?

**Decisión**: ocupan celdas con motivo declarado, no filas ausentes.

**Rationale**: una fila ausente es indistinguible de un olvido. Un motivo declarado es
información: alimenta el diagnóstico y hace que la verificación pase por una razón explícita.
Es el mismo criterio que el registro de capacidades ya aplica a la guardia de plan de un agente.
