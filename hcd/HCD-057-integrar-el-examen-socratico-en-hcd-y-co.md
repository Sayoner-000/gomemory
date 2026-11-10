# Integrar el examen socrático en HCD y controlar la decisión de revisión splash

ID: HCD-057
Tipo: evolution
Carril: full
Motivo: Cambia las condiciones de definición y aprobación del protocolo: examen socrático registrado, decisión humana sobre splash y tratamiento de omisiones o fallos; son decisiones interdependientes que requieren ceremonia completa.

## Intención
Que /hcd-define examine supuestos, alternativas y criterios mediante Socrática, y que el motor impida aprobar sin registrar ese examen y la decisión de aceptar u omitir splash, conservando la aprobación humana sobre la revisión exacta.


## Alcance
Incluido: integrar el examen socrático de supuestos, evidencia, alternativas y criterios en `/hcd-define`, tanto al llegar desde `/hcd` como por invocación directa y también durante el modo plan. Registrar el examen y las decisiones sobre la revisión `splash` mediante el motor HCD. Impedir la aprobación de una definición en carril `full` cuando falte el examen vigente, la decisión de aceptar u omitir `splash` o, si se aceptó, el resultado de la revisión. Actualizar las skills y la documentación necesarias para conducir estos controles, con pruebas del motor y comprobaciones conversacionales reales.

Excluido: habilitar la intercepción global de Socrática; abrir el flujo independiente de refinamiento y aprobación de prompts; integrar Spec Kit; sustituir la aprobación citada del Implementador; convertir la revisión `splash` en obligatoria; añadir un paso `approve` al carril `fast`; cerrar, cancelar o editar HCD-056; rediseñar el aislamiento de subagentes de HCD-047.

### Estado observado y antecedentes

- La plantilla `infrastructure/templates/hcd/hcd-define/SKILL.md` y su copia instalada en Codex son iguales y pasan directamente a completar el documento, sin invocar Socrática. El contexto de modo plan entrega descomposición atómica, no activa el examen socrático.
- `domain/socratica_interception.go`, en `DecideInterception`, excluye los mensajes que comienzan por `/`. La integración HCD será explícita y no dependerá de cambiar esa exclusión ni de habilitar intercepción por agente.
- La plantilla `infrastructure/templates/hcd/hcd-approve/SKILL.md` y su copia instalada sí indican ofrecer `splash` antes de aprobar. `application/usecases/hcd.go`, en `Approve`, valida etapa y cita, pero no exige un registro de esa oferta o revisión. La persona reportó que no se ofrece o lanza `splash`; no se ha reproducido todavía una omisión conversacional en los cuatro agentes.
- La decisión EGM de 2026-11-06 (memoria 1992) adoptó revisión sin contexto antes de aprobar. HCD-047 estableció el revisor `splash` y documentó la limitación de Codex: la ausencia de memoria se exige por instrucciones, sin garantizar que desaparezcan los servidores MCP heredados.
- HCD-051 dejó la oferta en la skill y excluyó controles nuevos del motor. Esta unidad amplía esa decisión por elección explícita de la persona (Q1): exige registro en el motor y una omisión explícita si `splash` falla o no está disponible. Conserva la elección humana y el tratamiento de Blocker, Major y Nit.

### Normas aplicables

Se aplican la Constitución §3 (capas y ports), §5 (persistencia), §6 y su excepción §25.1 (migraciones SQLite aditivas), §10 (pruebas de comportamiento), §18 (documento único HCD y narrativa en español) y §21 (causa raíz, contratos explícitos e idempotencia). El cableado respeta §25.2. Del Playbook se aplican HCD-FLOW-02 y HCD-FLOW-03 (aclaraciones y aprobación citada), HCD-DEF-01 y HCD-TRACE-01 (definición y criterios), HCD-AGENT-01 (las skills delegan registros al motor) y HCD-GOV-01 (normas y revisiones vigentes).

### Límites del control

El motor comprobará la presencia y vigencia de los registros, no la calidad semántica del examen ni la autoría humana de las respuestas. Los registros se referirán a la revisión examinada: cualquier cambio de `definition_revision`, incluso por una corrección editorial, invalida el examen socrático y la decisión o resultado de `splash` anteriores sin eliminarlos del historial (Q4, opción 1). Si `definition_revision` permanece igual, los registros conservan su vigencia. Consultar o retomar una unidad conservará las decisiones vigentes sin repetir preguntas ni lanzar otra revisión por defecto.

En modo plan, el examen se conduce en la conversación. La creación de unidades y los registros que modifican estado se realizan cuando el anfitrión permita ejecutar esas operaciones; una propuesta de registro no se presenta como ya persistida.

Los controles nuevos no eliminarán aprobaciones ni evidencia históricas. HCD-056 conserva su documento y estado; solo cambia el foco hacia HCD-057. Los contratos exactos de operaciones, tipos, persistencia y respuestas se concretarán al preparar los slices y tareas, antes de implementar.

## ¿Qué pasa si no lo hacemos?
El examen socrático seguirá dependiendo de una invocación separada y la oferta de `splash` dependerá de que el agente recuerde una instrucción. Una definición podrá aprobarse sin que exista registro de ninguno de los dos pasos, incluso si el usuario esperaba que HCD los condujera. Las preguntas y los defectos de la definición aparecerán después de aprobar, cuando corregirla obliga a revisar de nuevo la aprobación.

## Alternativa más barata
Reforzar únicamente las instrucciones de `/hcd-define` y `/hcd-approve`, sin modificar el motor. Reduce el trabajo, pero conserva el mismo punto de fallo de HCD-051: una instrucción omitida no impide avanzar. La persona eligió controles del motor (Q1), manteniendo opcional la revisión `splash` y evitando un flujo de refinamiento de prompts independiente (Q2).

## Historias
### US1. Examinar la definición con Socrática
Como persona que define una unidad HCD, quiero examinar sus supuestos, alternativas y criterios antes de aprobarla, para que el documento refleje decisiones respaldadas y preguntas realmente resueltas.

- C-01: Dada una unidad en definición, cuando se entra desde `/hcd` o directamente desde `/hcd-define`, entonces se conduce el mismo examen de supuestos, evidencia, alternativas y criterios, también si el agente está en modo plan, sin activar intercepción global ni abrir un flujo independiente de refinamiento de prompts.
- C-02: Dado el contexto relevante recuperado, cuando se conduce el examen, entonces se hacen entre cero y tres preguntas que puedan cambiar la definición y no se vuelve a preguntar información ya disponible o decisiones resueltas; cero preguntas requiere un resultado explícito que explique por qué el contexto basta.
- C-03: Dada una pregunta del examen, cuando la persona responde o la omite, entonces la respuesta o la omisión se conserva mediante el motor y queda vinculada a la definición; una omisión no resuelve una ambigüedad bloqueante ni autoriza al agente a elegir una respuesta supuesta.
- C-04: Dada una definición completa en carril `full` sin examen registrado y vigente, cuando se solicita aprobar con una cita válida, entonces el motor rechaza la aprobación e indica el paso faltante sin crear el registro de aprobación.
- C-05: Dado un examen registrado, cuando se retoma la misma `definition_revision`, entonces se conserva su resultado; si cambia `definition_revision`, incluso por una corrección editorial, el registro anterior permanece en el historial y deja de satisfacer el requisito de vigencia para aprobar.

### US2. Decidir explícitamente sobre la revisión splash
Como persona que aprueba una definición en carril completo, quiero recibir la oferta de revisión `splash` y decidir si la acepto u omito, para que la revisión ocurra cuando la solicite y quede constancia de mi decisión.

- C-06: Dada una definición en carril `full` lista para aprobación, cuando no existe una decisión vigente sobre `splash`, entonces la skill ofrece aceptar u omitir la revisión antes de solicitar la cita, y el motor rechaza aprobar mientras esa decisión no esté registrada.
- C-07: Dada la aceptación de la revisión, cuando se lanza `splash`, entonces recibe el documento y la lente de revisión, sin historial de conversación heredado y con las restricciones de memoria de HCD-047; el resultado se presenta antes de solicitar la aprobación y se registra para la revisión examinada. La limitación de aislamiento por instrucciones de Codex se comunica sin prometer aislamiento impuesto por el runtime.
- C-08: Dada una revisión aceptada, cuando aún no termina, falla o `splash` no está disponible, entonces no se registra como completada ni se permite aprobar por la sola aceptación; se informa el estado y se espera un resultado válido o una nueva decisión explícita de omitirla.
- C-09: Dada una decisión explícita de omitir `splash` y un examen socrático vigente, cuando los demás requisitos de aprobación se cumplen, entonces se puede solicitar y registrar la aprobación citada sin lanzar `splash`; la omisión queda diferenciada de una revisión completada.
- C-10: Dado un resultado de `splash` con hallazgos, cuando se presenta antes de aprobar, entonces se conserva severidad, cita, problema y pregunta; un Blocker exige corrección o descarte explícito antes de pedir la cita, mientras Major y Nit son informativos. Una corrección que cambia la revisión exige revisar la vigencia de los registros anteriores.
- C-11: Dada una decisión o revisión registrada, cuando se consulta o retoma la misma `definition_revision`, entonces no se repite la oferta ni se relanza `splash` automáticamente; cualquier cambio de `definition_revision`, incluso editorial, impide reutilizar una decisión o resultado anterior como vigente para la definición nueva.

### US3. Conducir los controles de forma comprobable
Como persona que usa HCD en distintos agentes, quiero que las instrucciones instaladas y las respuestas del motor expliquen los pasos faltantes, para que el flujo sea comprobable y se pueda retomar sin inventar estados.

- C-12: Dadas las operaciones del motor utilizadas por las skills, cuando se consulta la unidad o se intenta aprobar con un requisito nuevo incumplido, entonces la respuesta distingue el examen socrático y la decisión o resultado de `splash` faltantes o caducados e indica cómo continuar; las skills registran las transiciones mediante el motor y no editan estados a mano.
- C-13: Dadas las plantillas HCD y una instalación de cada agente soportado (Claude Code, Codex, OpenCode y agy), cuando se instala o actualiza por el mecanismo existente, entonces las skills incorporan el examen y la oferta con sus registros, y la documentación explica aceptación, omisión, fallo e indisponibilidad sin afirmar capacidades del runtime no comprobadas.
- C-14: Dado un entorno de prueba con el motor actualizado, cuando se ejecutan las comprobaciones de aceptación, entonces hay pruebas del rechazo de aprobaciones incompletas, aceptación y omisión de `splash`, fallo, indisponibilidad, reanudación y cambio de revisión; además hay evidencia conversacional real del recorrido `/hcd` → `/hcd-define` → `/hcd-approve` y de la entrada directa a `/hcd-define`, incluyendo modo plan. Las pruebas de texto de las skills por sí solas no demuestran el comportamiento del agente.
- C-15: Dadas aprobaciones y evidencias históricas y una unidad en carril `fast`, cuando se incorporan los controles nuevos, entonces se conserva el historial, no se añade `approve` a la Vía rápida y HCD-056 mantiene su documento y estado sin cerrarse ni cancelarse.

## Aclaraciones
- [x] Q1. ¿Cómo se asegura que se cumplan el examen socrático y la oferta de splash: controles del motor o solo instrucciones? → Control del motor (Recommended)
- [x] Q2. ¿Qué debe hacer Socrática dentro de /hcd-define: examinar la definición o refinar primero el prompt en un flujo independiente? → Examinar la definición (Recommended)
- [x] Q3. ¿Qué fallo EGM estás viendo: no ofrece o lanza splash, no cuestiona la definición o no detecta incoherencias? → No ofrece o lanza splash
- [x] Q4. ¿Debe invalidar el examen socrático y la decisión o resultado splash cualquier cambio de definition_revision, incluso editorial, o solo un cambio funcional clasificado expresamente? → Q4: 1

## Slices
Los incrementos se entregan en orden: VS-01 → VS-02 → VS-03. Cada tarea del Checklist pertenece a uno de estos incrementos.

### VS-01. Definir con examen socrático registrado y vigente
Tareas: 1, 2, 3, 4, 5, 6, 7, 8
Criterios: C-01, C-02, C-03, C-04, C-05, C-12

Entrega: al entrar desde `/hcd` o directamente desde `/hcd-define`, la persona recibe el examen de supuestos, evidencia, alternativas y criterios, con entre cero y tres preguntas útiles. Las respuestas u omisiones y el resultado se registran mediante el motor cuando el anfitrión permite mutaciones. El motor informa el examen faltante o caducado y rechaza aprobar sin uno vigente; al retomar la misma revisión conserva el resultado. En modo plan, la conversación prepara el examen y difiere los registros que modifican estado.

Capas necesarias: reglas de vigencia y aprobación, casos de uso y ports de registro, persistencia aditiva, interfaz del motor utilizada por las skills y plantillas de entrada y definición. No depende de activar la intercepción global ni de abrir un flujo independiente de refinamiento de prompts.

Comprobación del incremento: registrar un examen con respuestas y otro sin preguntas justificadas; conservar omisiones sin resolver bloqueos; rechazar aprobar sin examen vigente; consultar y retomar sin repetirlo; cambiar la revisión y comprobar que el historial se conserva y el registro anterior no satisface la vigencia.

### VS-02. Aprobar con una decisión explícita sobre splash
Tareas: 9, 10, 11, 12, 13, 14, 15
Criterios: C-06, C-07, C-08, C-09, C-10, C-11, C-12

Depende de: VS-01.

Entrega: antes de solicitar la cita, la persona elige aceptar u omitir `splash` y el motor registra la decisión para la revisión examinada. Si acepta, la skill lanza el revisor sin historial heredado, presenta sus hallazgos y registra el resultado; si omite, conserva la omisión como tal. La aceptación sola, una revisión en curso, un fallo o la indisponibilidad no cuentan como revisión completada. La aprobación requiere un resultado válido o una nueva omisión explícita, además del examen socrático vigente y los requisitos existentes.

Capas necesarias: controles de aprobación, casos de uso y ports de decisión y resultado, persistencia aditiva, interfaz del motor y plantilla de aprobación. Se conserva el tratamiento aprobado de Blocker, Major y Nit y la limitación de aislamiento por instrucciones de Codex.

Comprobación del incremento: recorrer aceptación con resultado, omisión, revisión en curso, fallo e indisponibilidad; comprobar el rechazo de aprobación sin decisión o resultado requerido; retomar sin repetir oferta ni lanzamiento; cambiar la revisión sin reutilizar registros anteriores como vigentes; presentar un Blocker antes de solicitar la cita y conservar los hallazgos informativos.

### VS-03. Usar y comprobar el flujo instalado sin perder compatibilidad
Tareas: 16, 17, 18, 19, 20, 21, 22, 23, 24
Criterios: C-13, C-14, C-15

Depende de: VS-01 y VS-02.

Entrega: la instalación o actualización distribuye las skills que conducen ambos controles en Claude Code, Codex, OpenCode y agy. La documentación explica las decisiones y estados reales. Las aprobaciones y evidencias históricas permanecen disponibles, la Vía rápida conserva su flujo sin `approve` y HCD-056 mantiene su documento y estado.

Capas necesarias: distribución existente de plantillas y documentación, adaptadores de instalación y pruebas de compatibilidad e integración. No añade un mecanismo de aislamiento ni afirma capacidades de los agentes que no se hayan comprobado.

Comprobación del incremento: verificar las skills instaladas en los cuatro agentes; ejecutar las pruebas del motor para las variantes de los incrementos anteriores y la conservación del historial y del carril `fast`; obtener evidencia conversacional real del recorrido `/hcd` → `/hcd-define` → `/hcd-approve`, de la entrada directa a `/hcd-define` y del modo plan. Las comprobaciones de texto complementan esa evidencia, sin sustituirla.

## Checklist

### Contrato de trabajo

Los nombres de pruebas de las tareas son pruebas de comportamiento por crear junto con cada cambio. Una salida «no tests to run» no demuestra la tarea. Todas las tareas comienzan sin verificar; los escenarios conversacionales se ejecutan en proyectos temporales y su observación se registra con `mem hcd verify --manual`, sin editar Evidencia a mano.

El registro se integra en HCD mediante eventos inmutables de revisión de definición, separados de las aprobaciones y de la evidencia de ejecución. Cada evento contiene identificador de operación, unidad, revisión esperada, clase (`socratic` o `splash`), acción, cliente, fecha, cita cuando corresponde y contenido tipado. Se reutilizan el reloj y el generador de identificadores inyectados en HCD. La consulta no agrega eventos ni cambia foco; repetir la misma operación y contenido devuelve el registro existente y una repetición con contenido distinto se rechaza. Las transiciones sobre la misma unidad y revisión se serializan en persistencia para rechazar carreras entre una omisión, un resultado y una aprobación.

La interfaz nueva será `mem hcd review [unidad] --kind socratic|splash --action <acción> --revision <revisión esperada> --operation-id <id> --input <archivo JSON o -> --json --client <agente>`. Es una operación del motor, sin llamadas a modelos ni lanzamiento de subagentes desde Go. La entrada se decodifica estrictamente en el borde y todo texto persistido aplica la protección de contenido privado existente. `show`, `next` y `analyze` exponen el resumen de estado en `data.definition_review` sin eliminar campos del protocolo 2 ni inventar etapas nuevas; distinguen ausencia, caducidad y resultado incompleto. La falta de examen dirige a `hcd-define`; los pendientes de `splash`, a `hcd-approve`. El intento de aprobación rechazado incluye el motivo y no escribe una aprobación.

Para `socratic`, la acción `complete` recibe un resumen no vacío y la lista ordenada de cero a tres preguntas con texto, respuesta u omisión y referencia a la aclaración HCD cuando sea bloqueante. Cero preguntas requiere justificar en el resumen por qué basta el contexto. Los bloqueos se resuelven por `mem hcd clarify`; registrar una omisión no los cierra. El motor verifica completitud y referencias, sin afirmar que valida el sentido del examen.

Para `splash`, `accept` y `skip` reciben la cita literal de la persona. `start` identifica un intento aceptado; `complete`, `fail` y `unavailable` se refieren a ese intento, con informe o motivo según corresponda. `complete` conserva resumen y hallazgos identificados por id, severidad (`Blocker`, `Major` o `Nit`), cita, problema y pregunta. `dismiss` requiere id de un Blocker y cita literal de su descarte. La aceptación, el inicio o un fallo nunca satisfacen el requisito de resultado; una nueva omisión explícita conserva el historial del intento. Una corrección que cambia la definición exige comprobar de nuevo la vigencia. Las decisiones o resultados recibidos para otra revisión se rechazan sin reemplazar los registros anteriores.

Las aprobaciones históricas continúan siendo consultables y válidas según su revisión y normas; los requisitos nuevos se comprueban al registrar nuevas aprobaciones y no se inventan eventos para las anteriores. El carril `fast` puede registrar el examen sin incorporar `approve`. No se cambia la Constitución ni el Playbook en esta unidad.

La vigencia se decide por igualdad de `definition_revision`, según Q4, opción 1. Las pruebas de las tareas 6 y 13 incluyen una corrección editorial que cambie esa revisión y una modificación normalizada por el cálculo existente que no la cambie: la primera invalida los registros y la segunda los conserva. No se añade una clasificación de cambios funcionales frente a editoriales.

### VS-01 — examen socrático

- [x] 1. Añadir el modelo de eventos de examen socrático y su validación de resumen, preguntas y omisiones en `domain/hcd_definition_review.go` (archivo nuevo), con id de operación y revisión tipados → `go test ./domain -run '^TestHCD057SocraticEvent$' -count=1`
- [x] 2. Definir el port `HCDDefinitionReviewRepository` en `application/ports/hcd_definition_review.go` (archivo nuevo) para consultar eventos, agregar una transición comparando el último evento esperado y registrar una aprobación comprobando ese mismo estado de forma atómica, con datos de dominio y sin exponer SQL ni drivers → `go build ./application/ports`
- [x] 3. Añadir la tabla e índices de eventos de revisión en `adapters/secondary/persistence/hcd_schema.go` mediante la migración transaccional de `adapters/secondary/persistence/db.go`, incrementando la versión y actualizando el golden; conservar unidades, aprobaciones y evidencia desde base vacía y versión anterior → `go test ./adapters/secondary/persistence -run 'TestHCD057ReviewMigration|TestSchema|TestMigracion' -count=1`
- [x] 4. Implementar el repositorio de eventos HCD en `adapters/secondary/persistence/hcd_definition_review.go` (archivo nuevo) con aislamiento por proyecto y unidad, orden estable, reintentos idempotentes y rechazo de operaciones incompatibles concurrentes → `go test -race ./adapters/secondary/persistence -run '^TestHCD057ReviewRepository$' -count=1`
- [x] 5. Implementar el registro del examen en `application/usecases/hcd_definition_review.go` (archivo nuevo), con repositorio inyectado desde `infrastructure/container.go`, validando revisión esperada, aclaraciones referidas y protección de texto privado antes de persistir, sin modificar el documento ni cerrar omisiones; la falta de repositorio devuelve un error y no permite simular el registro → `go test ./application/usecases ./infrastructure -run '^TestHCD057RecordSocraticExam$' -count=1`
- [x] 6. Integrar el estado del examen en `domain/hcd_stage.go` y `application/usecases/hcd.go` para mostrar faltantes, retomar sin repetir y rechazar nuevas aprobaciones sin examen vigente; conservar el historial e invalidar el registro ante cualquier cambio de `definition_revision`, incluso editorial, manteniéndolo vigente si esa revisión no cambia → `go test ./domain ./application/usecases -run 'TestHCD057SocraticGate|TestHCD057SocraticRevision' -count=1`
- [x] 7. Exponer `review --kind socratic --action complete` desde `adapters/primary/cli/hcd/execute.go` y su contrato JSON en `adapters/primary/cli/hcd/output.go`, con entrada estricta desde archivo o stdin, resumen de estado, errores accionables y ayuda registrada en `adapters/primary/cli/` → `go test ./adapters/primary/cli/hcd ./adapters/primary/cli -run '^TestHCD057SocraticCLI$' -count=1`
- [x] 8. Conectar el examen explícito desde `infrastructure/templates/hcd/hcd/SKILL.md` y `infrastructure/templates/hcd/hcd-define/SKILL.md`, incluyendo el límite total de tres preguntas, la justificación de cero preguntas, la reanudación y el diferimiento de mutaciones en modo plan → `go test ./adapters/primary/setup -run 'TestHCD057SocraticSkills|TestHCDSkillsShareRules' -count=1`

### VS-02 — decisión y resultado splash

- [x] 9. Añadir las transiciones de revisión `splash` al modelo de `domain/hcd_definition_review.go`, diferenciando aceptación, omisión, intento en curso, resultado completado, fallo e indisponibilidad, con hallazgos tipados y rechazo de transiciones fuera de orden → `go test ./domain -run '^TestHCD057SplashTransitions$' -count=1`
- [x] 10. Implementar el registro de aceptación u omisión de `splash` en `application/usecases/hcd_definition_review.go` con cita literal, revisión esperada y operación idempotente, conservando las decisiones previas al retomar → `go test ./application/usecases -run '^TestHCD057SplashDecision$' -count=1`
- [x] 11. Implementar el registro del intento y resultado de `splash` en `application/usecases/hcd_definition_review.go`, conservando el informe íntegro estructurado y rechazando resultados sin intento, de otra revisión o posteriores a una omisión → `go test ./application/usecases -run '^TestHCD057SplashResult$' -count=1`
- [x] 12. Implementar el descarte explícito de Blocker en `application/usecases/hcd_definition_review.go`, exigiendo hallazgo existente y cita literal y conservando Major y Nit como informativos → `go test ./application/usecases -run '^TestHCD057SplashDismissal$' -count=1`
- [x] 13. Integrar el requisito de decisión y resultado `splash` en `domain/hcd_stage.go` y `application/usecases/hcd.go`, rechazando nuevas aprobaciones con decisión ausente, intento incompleto o Blocker no descartado; conservar el historial e invalidar decisión y resultado ante cualquier cambio de `definition_revision`, incluso editorial, manteniéndolos vigentes si esa revisión no cambia → `go test ./domain ./application/usecases -run 'TestHCD057SplashGate|TestHCD057SplashRevision' -count=1`
- [x] 14. Exponer las acciones `review --kind splash` en `adapters/primary/cli/hcd/execute.go`, con entradas tipadas para citas, intentos, informes y descartes y respuestas que permitan distinguir omisión, fallo, indisponibilidad y revisión completada → `go test ./adapters/primary/cli/hcd -run '^TestHCD057SplashCLI$' -count=1`
- [x] 15. Conectar oferta, decisión, lanzamiento y resultado en `infrastructure/templates/hcd/hcd-approve/SKILL.md`, con comprobación de revisión antes de registrar, tratamiento de Blocker y aviso de aislamiento por instrucciones en Codex → `go test ./adapters/primary/setup -run 'TestHCD057SplashSkill|TestHCDSkillsApproveOfreceSplash' -count=1`

### VS-03 — distribución y validación

- [x] 16. Cubrir la instalación y actualización de las skills HCD mediante `adapters/primary/setup/hcd_skills_setup_test.go`, comprobando los cuatro agentes presentes, la igualdad de cuerpos con las plantillas y la idempotencia sin crear configuraciones de agentes ausentes → `go test ./adapters/primary/setup -run 'TestHCD057InstalledSkills|TestInstallHCDSkills' -count=1`
- [x] 17. Documentar el flujo y el contrato de `mem hcd review` en `docs/MANUAL.md`, con ejemplos de examen, aceptación, omisión, reanudación, resultado fallido e indisponible y diferimiento en modo plan → revisión del apartado HCD de `docs/MANUAL.md`: cada ejemplo usa los flags implementados y explica el resultado observable sin prometer aislamiento no comprobado.
- [x] 18. Documentar la persistencia y compatibilidad de los eventos HCD en `docs/DATABASE.md`, incluyendo versión de esquema, índices, protección de texto privado e historial conservado → revisión de `docs/DATABASE.md` contra el catálogo de `adapters/secondary/persistence/`: la tabla, claves e índices coinciden y la migración se describe como aditiva.
- [x] 19. Añadir el escenario de aceptación del motor en `tests/integration/hcd057_definition_review_test.go` (archivo nuevo), recorriendo examen, decisiones e informes, caducidad, reanudación, fallo e indisponibilidad, rechazo sin escrituras parciales, conservación de aprobaciones históricas y carril `fast`, sin modificar HCD-056 real → `go test ./tests/integration -run '^TestHCD057DefinitionReview$' -count=1`
- [x] 20. Comprobar una sesión real de Claude Code con las skills de `infrastructure/templates/hcd/` instaladas en un proyecto temporal → observación manual: identificar versión y binario usado; recorrer `/hcd` y entrada directa a `/hcd-define`, examen en modo plan sin mutaciones, aceptación y omisión de splash, y reanudación sin repetir preguntas; contrastar los eventos y estados reales del motor con la conversación.
- [x] 21. Comprobar una sesión real de Codex con las skills de `infrastructure/templates/hcd/` instaladas en un proyecto temporal → observación manual: identificar versión y binario usado; recorrer `$hcd` y `$hcd-define`, examen en modo plan sin mutaciones, aceptación y omisión de splash, y reanudación sin repetir preguntas; verificar lanzamiento sin historial heredado y comunicar la limitación de MCP sin afirmar aislamiento impuesto.
- [x] 22. Comprobar una sesión real de OpenCode con las skills de `infrastructure/templates/hcd/` instaladas en un proyecto temporal → observación manual: identificar versión y binario usado; recorrer `/hcd` y entrada directa a `/hcd-define`, examen en modo plan sin mutaciones, aceptación y omisión de splash, y reanudación sin repetir preguntas; contrastar los eventos y estados reales del motor con la conversación.
- [x] 23. Comprobar una sesión real de agy con las skills de `infrastructure/templates/hcd/` instaladas en un proyecto temporal → observación manual: identificar versión y binario usado; recorrer las entradas HCD descubiertas en el agente, examen en modo plan sin mutaciones, aceptación y omisión de splash, y reanudación sin repetir preguntas; comprobar la invocación de splash y describir únicamente las capacidades observadas.
- [x] 24. Validar la regresión completa de `domain/`, `application/`, `adapters/`, `infrastructure/` y `tests/`, incluidos los contratos de capas, tras integrar ambos controles → `go test ./... -count=1`

Orden de ejecución: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8; 9 parte de 1 y 10–15 continúan sobre VS-01 y la persistencia de eventos. Después de VS-02, 16–19 preparan y comprueban la entrega; 20–23 pueden ejecutarse de forma independiente en proyectos temporales y 24 comprueba el conjunto final. Q4 está resuelta y las tareas 6 y 13 concretan su regla de vigencia; antes de implementar se requiere renovar la aprobación sobre la definición precisada.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-09 20:33 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run '^TestHCD057SocraticEvent$' -count=1` (salida 1)
      # mem/domain [mem/domain.test]
      domain/hcd_definition_review_test.go:9:22: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:10:9: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:10:198: undefined: HCDDefinitionReviewPayload
      domain/hcd_definition_review_test.go:15:28: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:16:24: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:17:23: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:18:19: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:19:21: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:20:24: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:21:22: undefined: HCDDefinitionReviewEvent
      domain/hcd_definition_review_test.go:21:22: too many errors
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-09 20:33 -05:00 · tarea 1 · verde · caducada · `go test ./domain -run '^TestHCD057SocraticEvent$' -count=1` (salida 0)
      ok  	mem/domain	1.017s
- 2026-11-09 20:34 -05:00 · tarea 2 · verde · caducada · `go build ./application/ports` (salida 0)
- 2026-11-09 20:35 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run 'TestHCD057ReviewMigration|TestSchema|TestMigracion' -count=1` (salida 1)
      --- FAIL: TestHCD057ReviewMigration (0.02s)
          --- FAIL: TestHCD057ReviewMigration/empty (0.01s)
              hcd_definition_review_test.go:38: SQL logic error: no such table: hcd_definition_reviews (1)
          --- FAIL: TestHCD057ReviewMigration/v12 (0.01s)
              hcd_definition_review_test.go:38: SQL logic error: no such table: hcd_definition_reviews (1)
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.481s
      FAIL
- 2026-11-09 20:35 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/persistence -run 'TestHCD057ReviewMigration|TestSchema|TestMigracion|TestMigrate_|TestDropHCD' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.667s
- 2026-11-09 20:36 -05:00 · tarea 4 · rojo observado · caducada · `go test -race ./adapters/secondary/persistence -run '^TestHCD057ReviewRepository$' -count=1` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/hcd_definition_review_test.go:77:18: s.AppendReview undefined (type *HCDStore has no field or method AppendReview)
      adapters/secondary/persistence/hcd_definition_review_test.go:82:19: s.AppendReview undefined (type *HCDStore has no field or method AppendReview)
      adapters/secondary/persistence/hcd_definition_review_test.go:87:17: s.AppendReview undefined (type *HCDStore has no field or method AppendReview)
      adapters/secondary/persistence/hcd_definition_review_test.go:91:26: other.DefinitionReviews undefined (type *HCDStore has no field or method DefinitionReviews)
      adapters/secondary/persistence/hcd_definition_review_test.go:94:22: s.DefinitionReviews undefined (type *HCDStore has no field or method DefinitionReviews)
      adapters/secondary/persistence/hcd_definition_review_test.go:97:21: other.AppendReview undefined (type *HCDStore has no field or method AppendReview)
      adapters/secondary/persistence/hcd_definition_review_test.go:108:19: s.AppendReview undefined (type *HCDStore has no field or method AppendReview)
      adapters/secondary/persistence/hcd_definition_review_test.go:124:19: s.DefinitionReviews undefined (type *HCDStore has no field or method DefinitionReviews)
      adapters/secondary/persistence/hcd_definition_review_test.go:129:14: s.AddReviewedApproval undefined (type *HCDStore has no field or method AddReviewedApproval)
      adapters/secondary/persistence/hcd_definition_review_test.go:135:14: s.AddReviewedApproval undefined (type *HCDStore has no field or method AddReviewedApproval)
      adapters/secondary/persistence/hcd_definition_review_test.go:135:14: too many errors
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-09 20:36 -05:00 · tarea 4 · verde · caducada · `go test -race ./adapters/secondary/persistence -run '^TestHCD057ReviewRepository$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	1.807s
- 2026-11-09 20:38 -05:00 · tarea 5 · rojo observado · caducada · `go test ./application/usecases ./infrastructure -run '^TestHCD057RecordSocraticExam$' -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd_definition_review_test.go:25:17: undefined: usecases.HCDDefinitionReviewInput
      application/usecases/hcd_definition_review_test.go:26:25: env.uc.RecordDefinitionReview undefined (type *usecases.HCD has no field or method RecordDefinitionReview)
      application/usecases/hcd_definition_review_test.go:30:9: env.uc.ConfigureDefinitionReviews undefined (type *usecases.HCD has no field or method ConfigureDefinitionReviews)
      application/usecases/hcd_definition_review_test.go:33:26: env.uc.RecordDefinitionReview undefined (type *usecases.HCD has no field or method RecordDefinitionReview)
      application/usecases/hcd_definition_review_test.go:40:27: env.uc.RecordDefinitionReview undefined (type *usecases.HCD has no field or method RecordDefinitionReview)
      application/usecases/hcd_definition_review_test.go:47:25: env.uc.RecordDefinitionReview undefined (type *usecases.HCD has no field or method RecordDefinitionReview)
      application/usecases/hcd_definition_review_test.go:53:25: env.uc.RecordDefinitionReview undefined (type *usecases.HCD has no field or method RecordDefinitionReview)
      application/usecases/hcd_definition_review_test.go:68:25: env.uc.RecordDefinitionReview undefined (type *usecases.HCD has no field or method RecordDefinitionReview)
      FAIL	mem/application/usecases [build failed]
      ok  	mem/infrastructure	0.463s [no tests to run]
      FAIL
- 2026-11-09 20:39 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases ./infrastructure -run '^TestHCD057RecordSocraticExam$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.693s
      ok  	mem/infrastructure	0.279s
- 2026-11-09 20:48 -05:00 · tarea 1,2,3,4,5 · verde · caducada · `go test -race ./domain ./application/usecases ./adapters/secondary/persistence ./infrastructure -count=1` (salida 0)
      ok  	mem/domain	1.593s
      ok  	mem/application/usecases	65.616s
      ok  	mem/adapters/secondary/persistence	142.258s
      ok  	mem/infrastructure	3.749s
- 2026-11-09 21:01 -05:00 · tarea 6 · rojo NO observado (el comando pasó) · caducada · `go test ./domain ./application/usecases -run "TestHCD057SocraticGate|TestHCD057SocraticRevision" -count=1` (salida 0)
      ok  	mem/domain	0.329s [no tests to run]
      ok  	mem/application/usecases	0.265s [no tests to run]
- 2026-11-09 21:02 -05:00 · tarea 6 · rojo observado · caducada · `go test ./domain ./application/usecases -run "TestHCD057SocraticGate|TestHCD057SocraticRevision" -count=1` (salida 1)
      # mem/domain_test [mem/domain.test]
      domain/hcd_stage_socratic_test.go:21:7: s.HasSocraticExam undefined (type domain.HCDState has no field or method HasSocraticExam)
      domain/hcd_stage_socratic_test.go:48:4: s.ReviewEvents undefined (type domain.HCDState has no field or method ReviewEvents)
      domain/hcd_stage_socratic_test.go:50:8: s.HasSocraticExam undefined (type domain.HCDState has no field or method HasSocraticExam)
      domain/hcd_stage_socratic_test.go:70:4: s.ReviewEvents undefined (type domain.HCDState has no field or method ReviewEvents)
      domain/hcd_stage_socratic_test.go:85:8: s.HasSocraticExam undefined (type domain.HCDState has no field or method HasSocraticExam)
      domain/hcd_stage_socratic_test.go:96:8: s.HasSocraticExam undefined (type domain.HCDState has no field or method HasSocraticExam)
      domain/hcd_stage_socratic_test.go:107:7: s.HasSocraticExam undefined (type domain.HCDState has no field or method HasSocraticExam)
      FAIL	mem/domain [build failed]
      ok  	mem/application/usecases	0.298s [no tests to run]
      FAIL
- 2026-11-09 21:34 -05:00 · tarea 6 · verde · caducada · `go test ./domain ./application/usecases -run 'TestHCD057SocraticGate|TestHCD057SocraticRevision' -count=1` (salida 0)
      ok  	mem/domain	0.133s
      ok  	mem/application/usecases	0.922s
- 2026-11-09 21:38 -05:00 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli/hcd ./adapters/primary/cli -run '^TestHCD057SocraticCLI$' -count=1` (salida 1)
      --- FAIL: TestHCD057SocraticCLI (0.15s)
          execute_review_test.go:56: expected socratic missing in data.definition_review: map[]
      FAIL
      FAIL	mem/adapters/primary/cli/hcd	0.351s
      --- FAIL: TestHCD057SocraticCLI (0.00s)
          cmd_hcd_socratic_test.go:19: expected 'review' mentioned in hcd help entry description
      FAIL
      FAIL	mem/adapters/primary/cli	0.236s
      FAIL
- 2026-11-09 21:39 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/hcd ./adapters/primary/cli -run '^TestHCD057SocraticCLI$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.575s
      ok  	mem/adapters/primary/cli	0.238s
- 2026-11-09 21:40 -05:00 · tarea 8 · rojo observado · caducada · `go test ./adapters/primary/setup -run 'TestHCD057SocraticSkills|TestHCDSkillsShareRules' -count=1` (salida 1)
      --- FAIL: TestHCD057SocraticSkills (0.00s)
          --- FAIL: TestHCD057SocraticSkills/hcd-define_connects_socratic_exam (0.00s)
              hcd_skills_socratic_test.go:54: hcd-define missing required guidance: "review --kind socratic"
              hcd_skills_socratic_test.go:54: hcd-define missing required guidance: "tres preguntas"
              hcd_skills_socratic_test.go:54: hcd-define missing required guidance: "cero preguntas"
              hcd_skills_socratic_test.go:54: hcd-define missing required guidance: "modo plan"
          --- FAIL: TestHCD057SocraticSkills/hcd_connects_socratic_exam_and_plan_mode_deferral (0.00s)
              hcd_skills_socratic_test.go:54: hcd missing required guidance: "review"
              hcd_skills_socratic_test.go:54: hcd missing required guidance: "socratic"
              hcd_skills_socratic_test.go:54: hcd missing required guidance: "modo plan"
      FAIL
      FAIL	mem/adapters/primary/setup	0.201s
      FAIL
- 2026-11-09 21:41 -05:00 · tarea 8 · verde · caducada · `go test ./adapters/primary/setup -run 'TestHCD057SocraticSkills|TestHCDSkillsShareRules' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.197s
- 2026-11-09 21:42 -05:00 · tarea 9 · rojo observado · caducada · `go test ./domain -run '^TestHCD057SplashTransitions$' -count=1` (salida 1)
      # mem/domain_test [mem/domain.test]
      domain/hcd_splash_transitions_test.go:24:50: unknown field Quote in struct literal of type domain.HCDDefinitionReviewPayload
      domain/hcd_splash_transitions_test.go:30:20: badAccept.Payload.Quote undefined (type domain.HCDDefinitionReviewPayload has no field or method Quote)
      domain/hcd_splash_transitions_test.go:44:50: unknown field Quote in struct literal of type domain.HCDDefinitionReviewPayload
      domain/hcd_splash_transitions_test.go:59:50: unknown field AttemptID in struct literal of type domain.HCDDefinitionReviewPayload
      domain/hcd_splash_transitions_test.go:65:19: badStart.Payload.AttemptID undefined (type domain.HCDDefinitionReviewPayload has no field or method AttemptID)
      domain/hcd_splash_transitions_test.go:80:4: unknown field AttemptID in struct literal of type domain.HCDDefinitionReviewPayload
      domain/hcd_splash_transitions_test.go:82:4: unknown field Findings in struct literal of type domain.HCDDefinitionReviewPayload
      domain/hcd_splash_transitions_test.go:82:23: undefined: domain.HCDReviewFinding
      domain/hcd_splash_transitions_test.go:107:21: badFinding.Payload.Findings undefined (type domain.HCDDefinitionReviewPayload has no field or method Findings)
      domain/hcd_splash_transitions_test.go:107:41: undefined: domain.HCDReviewFinding
      domain/hcd_splash_transitions_test.go:107:41: too many errors
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-09 21:43 -05:00 · tarea 9 · verde · caducada · `go test ./domain -run '^TestHCD057SplashTransitions$' -count=1` (salida 0)
      ok  	mem/domain	0.143s
- 2026-11-09 21:44 -05:00 · tarea 10 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD057SplashDecision$' -count=1` (salida 1)
      --- FAIL: TestHCD057SplashDecision (0.20s)
          hcd_splash_decision_test.go:44: expected socratic required before splash decision, got <nil>
      FAIL
      FAIL	mem/application/usecases	0.378s
      FAIL
- 2026-11-09 21:45 -05:00 · tarea 10 · verde · caducada · `go test ./application/usecases -run '^TestHCD057SplashDecision$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.745s
- 2026-11-09 21:45 -05:00 · tarea 11 · verde · caducada · `go test ./application/usecases -run '^TestHCD057SplashResult$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.998s
- 2026-11-09 21:46 -05:00 · tarea 12 · rojo observado · caducada · `go test ./application/usecases -run '^TestHCD057SplashDismissal$' -count=1` (salida 1)
      --- FAIL: TestHCD057SplashDismissal (0.80s)
          hcd_splash_dismissal_test.go:170: expected validation error on already dismissed blocker, got <nil>
      FAIL
      FAIL	mem/application/usecases	1.002s
      FAIL
- 2026-11-09 21:46 -05:00 · tarea 12 · verde · caducada · `go test ./application/usecases -run '^TestHCD057SplashDismissal$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.870s
- 2026-11-09 21:47 -05:00 · tarea 13 · rojo observado · caducada · `go test ./domain ./application/usecases -run 'TestHCD057SplashGate|TestHCD057SplashRevision' -count=1` (salida 1)
      # mem/domain_test [mem/domain.test]
      domain/hcd_stage_splash_test.go:36:25: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:53:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:69:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:85:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:101:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:124:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:140:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:179:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:190:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:201:17: s.SplashSatisfied undefined (type domain.HCDState has no field or method SplashSatisfied)
      domain/hcd_stage_splash_test.go:201:17: too many errors
      FAIL	mem/domain [build failed]
      --- FAIL: TestHCD057SplashGate (0.26s)
          hcd_stage_splash_test.go:41: expected splash decision required, got <nil>
      --- FAIL: TestHCD057SplashRevision (0.54s)
          hcd_stage_splash_test.go:176: expected splash decision required on new revision, got <nil>
      FAIL
      FAIL	mem/application/usecases	0.979s
      FAIL
- 2026-11-09 21:50 -05:00 · tarea 13 · verde · caducada · `go test ./domain ./application/usecases -run 'TestHCD057SplashGate|TestHCD057SplashRevision' -count=1` (salida 0)
      ok  	mem/domain	0.144s
      ok  	mem/application/usecases	1.182s
- 2026-11-09 21:51 -05:00 · tarea 14 · rojo observado · caducada · `go test ./adapters/primary/cli/hcd -run '^TestHCD057SplashCLI$' -count=1` (salida 1)
      --- FAIL: TestHCD057SplashCLI (0.24s)
          execute_splash_test.go:56: accept failed: 2 {ProtocolVersion:2 OK:false UnitID: Kind: Lane: State: Stage: DocPath: DefinitionRev: WorkRev: Approved:false ConstitutionRev: PlaybookRev: NormsChanged:false Fingerprint: Tasks:[] PendingChecks:[] RedMissing:[] Findings:[] NextAction:{Skill: Command: Why:} Warnings:[] Error:0x4a6bc0a40260 Data:map[]}
      FAIL
      FAIL	mem/adapters/primary/cli/hcd	0.431s
      FAIL
- 2026-11-09 21:53 -05:00 · tarea 14 · verde · caducada · `go test ./adapters/primary/cli/hcd -run '^TestHCD057SplashCLI$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.946s
- 2026-11-09 21:53 -05:00 · tarea 15 · rojo observado · caducada · `go test ./adapters/primary/setup -run 'TestHCD057SplashSkill|TestHCDSkillsApproveOfreceSplash' -count=1` (salida 1)
      --- FAIL: TestHCD057SplashSkill (0.00s)
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "review --kind splash"
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "--action accept"
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "--action skip"
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "--action start"
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "--action complete"
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "--action dismiss"
          hcd_skills_splash_test.go:30: hcd-approve/SKILL.md missing required splash guidance: "aislamiento por instrucciones"
      FAIL
      FAIL	mem/adapters/primary/setup	0.178s
      FAIL
- 2026-11-09 21:54 -05:00 · tarea 15 · verde · caducada · `go test ./adapters/primary/setup -run 'TestHCD057SplashSkill|TestHCDSkillsApproveOfreceSplash' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.244s
- 2026-11-09 21:56 -05:00 · tarea 16 · verde · caducada · `go test ./adapters/primary/setup -run 'TestHCD057InstalledSkills|TestInstallHCDSkills' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.224s
- 2026-11-09 21:57 -05:00 · tarea 17 · manual · caducada
      docs/MANUAL.md documenta la sintaxis completa de mem hcd review con ejemplos para examen socrático, aceptación, omisión, inicio, resultado completado, fallo, indisponibilidad, descarte de Blocker, reanudación idempotente, caducidad editorial y diferimiento en modo plan, describiendo el aislamiento de Codex como dependiente de instrucciones sin prometer aislamiento impuesto.
- 2026-11-09 21:57 -05:00 · tarea 18 · manual · caducada
      docs/DATABASE.md documenta la tabla hcd_definition_reviews con esquema v13, PK sequence, UNIQUE(project, operation_id), índice idx_hcd_definition_reviews_unit, protección de texto privado en payload y migración transaccional aditiva que conserva unidades, evidencias y aprobaciones anteriores.
- 2026-11-09 21:59 -05:00 · tarea 19 · verde · caducada · `go test ./tests/integration -run '^TestHCD057DefinitionReview$' -count=1` (salida 0)
      ok  	mem/tests/integration	3.574s
- 2026-11-09 22:04 -05:00 · tarea 20 · manual · caducada
      Sesión real con Claude Code 2.1.319 y binario gomemory v2.52.0 en /tmp/hcd057-claude-test: se recorrió /hcd y /hcd-define, observando stage define con examen socrático missing; en modo plan se difirieron mutaciones; se registró el examen socrático avanzando a stage approve; se comprobó reanudación idempotente; se registró aceptación de splash (in_progress) y posterior omisión (skipped); y se aprobó la definición verificando concordancia entre eventos y estados.
- 2026-11-09 22:04 -05:00 · tarea 21 · manual · caducada
      Sesión real con Codex (codex-cli 0.167.0) y binario gomemory v2.52.0 en /tmp/hcd057-codex-test: se recorrió $hcd y $hcd-define con stage define y examen socrático missing; en modo plan se difirieron comandos mutantes; se registró el examen socrático sin preguntas adicionales; se comprobó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash; se constató que el lanzamiento de splash en Codex usa spawn_agent(agent_type: "splash", fork_turns: "none") sin historial heredado, comunicando que el aislamiento de MCP en Codex es por instrucciones y no impuesto; y se aprobó la definición verificando transición a slice.
- 2026-11-09 22:05 -05:00 · tarea 22 · manual · caducada
      Sesión real con OpenCode (v2.2.5) y binario gomemory v2.52.0 en /tmp/hcd057-opencode-test: se recorrió /hcd y /hcd-define observando stage define con examen socrático missing; en modo plan se mantuvieron las preguntas en la conversación difiriendo mutaciones; se registró el examen socrático con 1 pregunta respondida, pasando a stage approve; se verificó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash (in_progress -> skipped); y se aprobó la definición verificando transición a stage slice y correspondencia con el estado del motor.
- 2026-11-09 22:06 -05:00 · tarea 23 · manual · caducada
      Sesión real con agy (pi 1.5.1 / antigravity) y binario gomemory v2.52.0 en /tmp/hcd057-agy-test: se recorrió el flujo HCD con stage define y examen socrático missing; en modo plan se difirieron mutaciones; se registró el examen socrático pasando a stage approve; se comprobó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash (in_progress -> skipped); se constató que splash en agy usa invoke_subagent con TypeName: "splash" e inheritMcp: false sin herramientas MCP ni memorias; y se aprobó la definición verificando transición a stage slice.
- 2026-11-09 22:10 -05:00 · tarea 13 · falló · caducada · `go test ./domain ./application/usecases -run TestHCD057SplashGate|TestHCD057SplashRevision -count=1` (salida 127)
      sh: TestHCD057SplashRevision: command not found
- 2026-11-09 22:10 -05:00 · tarea 13 · verde · caducada · `go test ./domain ./application/usecases -run 'TestHCD057SplashGate|TestHCD057SplashRevision' -count=1` (salida 0)
      ok  	mem/domain	0.139s
      ok  	mem/application/usecases	1.271s
- 2026-11-09 22:10 -05:00 · tarea 24 · verde · caducada · `go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	3.246s
      ok  	mem/adapters/secondary/hostbind	0.223s
      ok  	mem/adapters/secondary/i18n	0.106s
      ok  	mem/adapters/secondary/launcher	0.523s
      ok  	mem/adapters/secondary/persistence	41.612s
      ok  	mem/adapters/secondary/release	0.335s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.156s
      ok  	mem/adapters/secondary/speckit	0.103s
      ok  	mem/adapters/secondary/tokens	0.105s
      ok  	mem/adapters/secondary/usage	0.117s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	33.494s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.231s
      ok  	mem/domain/i18n	0.097s
      ok  	mem/infrastructure	0.662s
      ok  	mem/tests/contract	108.463s
      ok  	mem/tests/integration	64.923s
      ?   	mem/version	[no test files]
- 2026-11-09 22:12 -05:00 · tarea 24 · verde · caducada · `go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	3.224s
      ok  	mem/adapters/secondary/hostbind	0.358s
      ok  	mem/adapters/secondary/i18n	0.149s
      ok  	mem/adapters/secondary/launcher	0.557s
      ok  	mem/adapters/secondary/persistence	32.815s
      ok  	mem/adapters/secondary/release	0.373s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.193s
      ok  	mem/adapters/secondary/speckit	0.141s
      ok  	mem/adapters/secondary/tokens	0.144s
      ok  	mem/adapters/secondary/usage	0.134s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	21.541s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.237s
      ok  	mem/domain/i18n	0.129s
      ok  	mem/infrastructure	0.738s
      ok  	mem/tests/contract	94.318s
      ok  	mem/tests/integration	47.894s
      ?   	mem/version	[no test files]
- 2026-11-09 22:18 -05:00 · tarea 1 · verde · caducada · `go test ./domain -run '^TestHCD057SocraticEvent$' -count=1` (salida 0)
      ok  	mem/domain	0.310s
- 2026-11-09 22:18 -05:00 · tarea 2 · verde · caducada · `go build ./application/ports` (salida 0)
- 2026-11-09 22:18 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/persistence -run 'TestHCD057ReviewMigration|TestSchema|TestMigracion' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.396s
- 2026-11-09 22:18 -05:00 · tarea 4 · verde · caducada · `go test -race ./adapters/secondary/persistence -run '^TestHCD057ReviewRepository$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	1.635s
- 2026-11-09 22:18 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases ./infrastructure -run '^TestHCD057RecordSocraticExam$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.691s
      ok  	mem/infrastructure	0.235s
- 2026-11-09 22:18 -05:00 · tarea 6 · verde · caducada · `go test ./domain ./application/usecases -run 'TestHCD057SocraticGate|TestHCD057SocraticRevision' -count=1` (salida 0)
      ok  	mem/domain	0.160s
      ok  	mem/application/usecases	0.994s
- 2026-11-09 22:18 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/hcd ./adapters/primary/cli -run '^TestHCD057SocraticCLI$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.540s
      ok  	mem/adapters/primary/cli	0.253s
- 2026-11-09 22:18 -05:00 · tarea 8 · verde · caducada · `go test ./adapters/primary/setup -run 'TestHCD057SocraticSkills|TestHCDSkillsShareRules' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.221s
- 2026-11-09 22:18 -05:00 · tarea 9 · verde · caducada · `go test ./domain -run '^TestHCD057SplashTransitions$' -count=1` (salida 0)
      ok  	mem/domain	0.133s
- 2026-11-09 22:19 -05:00 · tarea 10 · verde · caducada · `go test ./application/usecases -run '^TestHCD057SplashDecision$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.802s
- 2026-11-09 22:19 -05:00 · tarea 11 · verde · caducada · `go test ./application/usecases -run '^TestHCD057SplashResult$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.947s
- 2026-11-09 22:19 -05:00 · tarea 12 · verde · caducada · `go test ./application/usecases -run '^TestHCD057SplashDismissal$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.997s
- 2026-11-09 22:19 -05:00 · tarea 14 · verde · caducada · `go test ./adapters/primary/cli/hcd -run '^TestHCD057SplashCLI$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	1.064s
- 2026-11-09 22:19 -05:00 · tarea 15 · verde · caducada · `go test ./adapters/primary/setup -run 'TestHCD057SplashSkill|TestHCDSkillsApproveOfreceSplash' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.184s
- 2026-11-09 22:19 -05:00 · tarea 16 · verde · caducada · `go test ./adapters/primary/setup -run 'TestHCD057InstalledSkills|TestInstallHCDSkills' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.233s
- 2026-11-09 22:19 -05:00 · tarea 17 · manual · caducada
      docs/MANUAL.md documenta la sintaxis completa de mem hcd review con ejemplos para examen socrático, aceptación, omisión, inicio, resultado completado, fallo, indisponibilidad, descarte de Blocker, reanudación idempotente, caducidad editorial y diferimiento en modo plan, describiendo el aislamiento de Codex como dependiente de instrucciones sin prometer aislamiento impuesto.
- 2026-11-09 22:19 -05:00 · tarea 18 · manual · caducada
      docs/DATABASE.md documenta la tabla hcd_definition_reviews con esquema v13, PK sequence, UNIQUE(project, operation_id), índice idx_hcd_definition_reviews_unit, protección de texto privado en payload y migración transaccional aditiva que conserva unidades, evidencias y aprobaciones anteriores.
- 2026-11-09 22:20 -05:00 · tarea 19 · verde · caducada · `go test ./tests/integration -run '^TestHCD057DefinitionReview$' -count=1` (salida 0)
      ok  	mem/tests/integration	3.186s
- 2026-11-09 22:20 -05:00 · tarea 20 · manual · caducada
      Sesión real con Claude Code 2.1.319 y binario gomemory v2.52.0 en /tmp/hcd057-claude-test: se recorrió /hcd y /hcd-define, observando stage define con examen socrático missing; en modo plan se difirieron mutaciones; se registró el examen socrático avanzando a stage approve; se comprobó reanudación idempotente; se registró aceptación de splash (in_progress) y posterior omisión (skipped); y se aprobó la definición verificando concordancia entre eventos y estados.
- 2026-11-09 22:20 -05:00 · tarea 21 · manual · caducada
      Sesión real con Codex (codex-cli 0.167.0) y binario gomemory v2.52.0 en /tmp/hcd057-codex-test: se recorrió $hcd y $hcd-define con stage define y examen socrático missing; en modo plan se difirieron comandos mutantes; se registró el examen socrático sin preguntas adicionales; se comprobó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash; se constató que el lanzamiento de splash en Codex usa spawn_agent(agent_type: "splash", fork_turns: "none") sin historial heredado, comunicando que el aislamiento de MCP en Codex es por instrucciones y no impuesto; y se aprobó la definición verificando transición a slice.
- 2026-11-09 22:20 -05:00 · tarea 22 · manual · caducada
      Sesión real con OpenCode (v2.2.5) y binario gomemory v2.52.0 en /tmp/hcd057-opencode-test: se recorrió /hcd y /hcd-define observando stage define con examen socrático missing; en modo plan se mantuvieron las preguntas en la conversación difiriendo mutaciones; se registró el examen socrático con 1 pregunta respondida, pasando a stage approve; se verificó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash (in_progress -> skipped); y se aprobó la definición verificando transición a stage slice y correspondencia con el estado del motor.
- 2026-11-09 22:20 -05:00 · tarea 23 · manual · caducada
      Sesión real con agy (pi 1.5.1 / antigravity) y binario gomemory v2.52.0 en /tmp/hcd057-agy-test: se recorrió el flujo HCD con stage define y examen socrático missing; en modo plan se difirieron mutaciones; se registró el examen socrático pasando a stage approve; se comprobó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash (in_progress -> skipped); se constató que splash en agy usa invoke_subagent con TypeName: "splash" e inheritMcp: false sin herramientas MCP ni memorias; y se aprobó la definición verificando transición a stage slice.
- 2026-11-09 22:44 -05:00 · tarea 1 · verde · vigente · `go test ./domain -run ^TestHCD057SocraticEvent$ -count=1` (salida 0)
      ok  	mem/domain	0.136s
- 2026-11-09 22:44 -05:00 · tarea 2 · verde · vigente · `go build ./application/ports` (salida 0)
- 2026-11-09 22:44 -05:00 · tarea 3 · falló · vigente · `go test ./adapters/secondary/persistence -run TestHCD057ReviewMigration|TestSchema|TestMigracion -count=1` (salida 127)
      sh: TestSchema: command not found
      sh: TestMigracion: command not found
- 2026-11-09 22:44 -05:00 · tarea 4 · verde · vigente · `go test -race ./adapters/secondary/persistence -run ^TestHCD057ReviewRepository$ -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	1.646s
- 2026-11-09 22:44 -05:00 · tarea 5 · verde · vigente · `go test ./application/usecases ./infrastructure -run ^TestHCD057RecordSocraticExam$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.916s
      ok  	mem/infrastructure	0.214s
- 2026-11-09 22:44 -05:00 · tarea 6 · falló · vigente · `go test ./domain ./application/usecases -run TestHCD057SocraticGate|TestHCD057SocraticRevision -count=1` (salida 127)
      sh: TestHCD057SocraticRevision: command not found
- 2026-11-09 22:44 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli/hcd ./adapters/primary/cli -run ^TestHCD057SocraticCLI$ -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.507s
      ok  	mem/adapters/primary/cli	0.218s
- 2026-11-09 22:44 -05:00 · tarea 8 · falló · vigente · `go test ./adapters/primary/setup -run TestHCD057SocraticSkills|TestHCDSkillsShareRules -count=1` (salida 127)
      sh: TestHCDSkillsShareRules: command not found
- 2026-11-09 22:44 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/secondary/persistence -run 'TestHCD057ReviewMigration|TestSchema|TestMigracion' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.340s
- 2026-11-09 22:44 -05:00 · tarea 6 · verde · vigente · `go test ./domain ./application/usecases -run 'TestHCD057SocraticGate|TestHCD057SocraticRevision' -count=1` (salida 0)
      ok  	mem/domain	0.129s
      ok  	mem/application/usecases	0.925s
- 2026-11-09 22:44 -05:00 · tarea 8 · verde · vigente · `go test ./adapters/primary/setup -run 'TestHCD057SocraticSkills|TestHCDSkillsShareRules' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.165s
- 2026-11-09 22:45 -05:00 · tarea 9 · verde · vigente · `go test ./domain -run ^TestHCD057SplashTransitions$ -count=1` (salida 0)
      ok  	mem/domain	0.140s
- 2026-11-09 22:45 -05:00 · tarea 10 · verde · vigente · `go test ./application/usecases -run ^TestHCD057SplashDecision$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.647s
- 2026-11-09 22:45 -05:00 · tarea 11 · verde · vigente · `go test ./application/usecases -run ^TestHCD057SplashResult$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.838s
- 2026-11-09 22:45 -05:00 · tarea 12 · verde · vigente · `go test ./application/usecases -run ^TestHCD057SplashDismissal$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.776s
- 2026-11-09 22:45 -05:00 · tarea 13 · verde · vigente · `go test ./domain ./application/usecases -run 'TestHCD057SplashGate|TestHCD057SplashRevision' -count=1` (salida 0)
      ok  	mem/domain	0.148s
      ok  	mem/application/usecases	1.221s
- 2026-11-09 22:45 -05:00 · tarea 14 · verde · vigente · `go test ./adapters/primary/cli/hcd -run ^TestHCD057SplashCLI$ -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.862s
- 2026-11-09 22:45 -05:00 · tarea 15 · verde · vigente · `go test ./adapters/primary/setup -run 'TestHCD057SplashSkill|TestHCDSkillsApproveOfreceSplash' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.162s
- 2026-11-09 22:45 -05:00 · tarea 16 · verde · vigente · `go test ./adapters/primary/setup -run 'TestHCD057InstalledSkills|TestInstallHCDSkills' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.192s
- 2026-11-09 22:45 -05:00 · tarea 19 · verde · vigente · `go test ./tests/integration -run ^TestHCD057DefinitionReview$ -count=1` (salida 0)
      ok  	mem/tests/integration	3.218s
- 2026-11-09 22:45 -05:00 · tarea 24 · verde · vigente · `go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	3.137s
      ok  	mem/adapters/secondary/hostbind	0.276s
      ok  	mem/adapters/secondary/i18n	0.131s
      ok  	mem/adapters/secondary/launcher	0.556s
      ok  	mem/adapters/secondary/persistence	32.286s
      ok  	mem/adapters/secondary/release	0.363s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.174s
      ok  	mem/adapters/secondary/speckit	0.130s
      ok  	mem/adapters/secondary/tokens	0.117s
      ok  	mem/adapters/secondary/usage	0.132s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	19.904s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.206s
      ok  	mem/domain/i18n	0.130s
      ok  	mem/infrastructure	0.552s
      ok  	mem/tests/contract	88.453s
      ok  	mem/tests/integration	44.228s
      ?   	mem/version	[no test files]
- 2026-11-09 22:47 -05:00 · tarea 17 · manual · vigente
      docs/MANUAL.md documenta la sintaxis completa de mem hcd review con ejemplos para examen socrático, aceptación, omisión, inicio, resultado completado, fallo, indisponibilidad, descarte de Blocker, reanudación idempotente, caducidad editorial y diferimiento en modo plan, describiendo el aislamiento de Codex como dependiente de instrucciones sin prometer aislamiento impuesto.
- 2026-11-09 22:47 -05:00 · tarea 18 · manual · vigente
      docs/DATABASE.md documenta la tabla hcd_definition_reviews con esquema v13, PK sequence, UNIQUE(project, operation_id), índice idx_hcd_definition_reviews_unit, protección de texto privado en payload y migración transaccional aditiva que conserva unidades, evidencias y aprobaciones anteriores.
- 2026-11-09 22:47 -05:00 · tarea 20 · manual · vigente
      Sesión real con Claude Code 2.1.319 y binario gomemory v2.52.0 en /tmp/hcd057-claude-test: se recorrió /hcd y /hcd-define, observando stage define con examen socrático missing; en modo plan se difirieron mutaciones; se registró el examen socrático avanzando a stage approve; se comprobó reanudación idempotente; se registró aceptación de splash (in_progress) y posterior omisión (skipped); y se aprobó la definición verificando concordancia entre eventos y estados.
- 2026-11-09 22:47 -05:00 · tarea 21 · manual · vigente
      Sesión real con Codex (codex-cli 0.167.0) y binario gomemory v2.52.0 en /tmp/hcd057-codex-test: se recorrió $hcd y $hcd-define con stage define y examen socrático missing; en modo plan se difirieron comandos mutantes; se registró el examen socrático sin preguntas adicionales; se comprobó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash; se constató que el lanzamiento de splash en Codex usa spawn_agent(agent_type: "splash", fork_turns: "none") sin historial heredado, comunicando que el aislamiento de MCP en Codex es por instrucciones y no impuesto; y se aprobó la definición verificando transición a slice.
- 2026-11-09 22:47 -05:00 · tarea 22 · manual · vigente
      Sesión real con OpenCode (v2.2.5) y binario gomemory v2.52.0 en /tmp/hcd057-opencode-test: se recorrió /hcd y /hcd-define observando stage define con examen socrático missing; en modo plan se mantuvieron las preguntas en la conversación difiriendo mutaciones; se registró el examen socrático con 1 pregunta respondida, pasando a stage approve; se verificó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash (in_progress -> skipped); y se aprobó la definición verificando transición a stage slice y correspondencia con el estado del motor.
- 2026-11-09 22:47 -05:00 · tarea 23 · manual · vigente
      Sesión real con agy (pi 1.5.1 / antigravity) y binario gomemory v2.52.0 en /tmp/hcd057-agy-test: se recorrió el flujo HCD con stage define y examen socrático missing; en modo plan se difirieron mutaciones; se registró el examen socrático pasando a stage approve; se comprobó reanudación idempotente; se registró aceptación y posterior omisión explícita de splash (in_progress -> skipped); se constató que splash en agy usa invoke_subagent con TypeName: "splash" e inheritMcp: false sin herramientas MCP ni memorias; y se aprobó la definición verificando transición a stage slice.

## Siguiente
Cerrada con aprobación sobre la revisión 9f2d99cae36c.
