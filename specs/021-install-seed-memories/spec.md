# Feature Specification: Instalación sin artefactos — reglas y constitución como memorias semilla

**Feature Directory**: `specs/021-install-seed-memories`

**Feature Branch**: `main` (sin rama dedicada; el hook `before_specify` de este proyecto no crea ramas)

**Created**: 2026-08-23

**Status**: Draft

**Input**: User description: "Optimización de `mem install`: cero archivos generados, semillas en memoria. Hoy `mem install` deja `speckit-constitution-gen.md`, `AGENTS.md`, `CLAUDE.md`, `.windsurf/` y `.cline/` en la raíz del proyecto destino. Nada de eso hace falta desde v1.9 porque el servidor MCP ya entrega el protocolo en la respuesta `initialize`. Se busca que `mem install` y `mem update` no generen ningún archivo de instrucciones ni de constitución, limpien los que dejaron instalaciones previas, y en su lugar siembren dos memorias: las reglas de trabajo como preferencia fijada (que `get_context()` emite íntegras) y la constitución como decisión de arquitectura (que `/constitution` aplica bajo demanda)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Las reglas de trabajo y la constitución viven en la memoria (Priority: P1)

Como persona que trabaja con un agente AI sobre un proyecto con gomemory, quiero que las reglas de trabajo del equipo y la constitución técnica estén guardadas en la memoria del proyecto —no en archivos sueltos del repositorio— para poder editarlas en un solo lugar y que el agente las reciba siempre, sin que dependan de que alguien mantenga un archivo Markdown sincronizado.

**Why this priority**: Es la pieza fundacional. Sin las semillas en la memoria, retirar los archivos de instrucciones sería una pérdida neta de funcionalidad. Con ellas, el resto de la feature solo quita duplicados.

**Independent Test**: En un proyecto donde gomemory nunca se usó, provocar la primera interacción con la memoria y comprobar que existen las dos memorias sembradas y que las reglas de trabajo aparecen completas al pedir el contexto del proyecto.

**Acceptance Scenarios**:

1. **Given** un proyecto sin memoria previa, **When** se usa gomemory por primera vez (instalación o primer arranque del servidor de memoria), **Then** quedan guardadas exactamente dos memorias semilla: las reglas de trabajo y la constitución, cada una identificada por una clave de tópico estable.
2. **Given** un proyecto con las semillas ya presentes, **When** se vuelve a instalar o a arrancar, **Then** ninguna de las dos se modifica ni se duplica.
3. **Given** que la persona editó el texto de la memoria de reglas, **When** se reinstala con una versión más nueva del binario, **Then** la edición sobrevive intacta: la plantilla del binario no pisa el contenido del usuario.
4. **Given** un proyecto con las semillas presentes, **When** el agente pide el contexto del proyecto, **Then** las reglas de trabajo aparecen en su propia sección y con el texto completo, sin recorte ni puntero a "leer más".
5. **Given** un proyecto con decenas de memorias más recientes que la semilla, **When** el agente pide el contexto, **Then** las reglas de trabajo siguen apareciendo: no se pierden por antigüedad.

---

### User Story 2 - Instalar y actualizar sin ensuciar el repositorio (Priority: P2)

Como persona que instala gomemory en un proyecto propio, quiero que la instalación no cree archivos ni carpetas en la raíz del repositorio, para no tener que revisar, ignorar o borrar a mano artefactos que no escribí yo y que además duplican información que el agente ya recibe por otra vía.

**Why this priority**: Es el motivo declarado de la feature y lo que la persona ve de inmediato, pero depende de que las semillas de la US1 ya cubran lo que esos archivos aportaban.

**Independent Test**: Instalar en un directorio vacío y comparar el listado de la raíz antes y después: el único elemento nuevo permitido es el binario y el directorio de memoria del proyecto.

**Acceptance Scenarios**:

1. **Given** un directorio de proyecto vacío, **When** se instala gomemory, **Then** no se crean archivos de instrucciones para agentes ni copia de la constitución en la raíz.
2. **Given** un directorio de proyecto vacío, **When** se instala gomemory, **Then** no se crean carpetas de configuración para agentes que la persona no pidió explícitamente.
3. **Given** una instalación recién hecha, **When** termina, **Then** el mensaje final explica dónde quedaron las reglas y la constitución y cómo se aplican, sin mencionar archivos que ya no se generan.
4. **Given** un proyecto ya instalado, **When** se actualiza la herramienta, **Then** el refresco de la integración se comporta igual que una instalación limpia: no reintroduce ningún archivo retirado.

---

### User Story 3 - Limpiar lo que dejaron las instalaciones anteriores (Priority: P3)

Como persona con proyectos instalados con versiones previas, quiero que al instalar o actualizar se retiren los artefactos que esas versiones dejaron, para no arrastrar indefinidamente archivos obsoletos que contradicen la memoria y gastan contexto.

**Why this priority**: Aporta el valor completo (el repositorio queda limpio de verdad), pero un proyecto nuevo ya se beneficia solo con la US2.

**Independent Test**: Preparar un directorio que simule una instalación antigua (con los archivos y carpetas que aquella versión generaba), instalar la versión nueva y comprobar que esos elementos desaparecieron.

**Acceptance Scenarios**:

1. **Given** un proyecto con archivos de instrucciones generados por una versión anterior, **When** se instala o actualiza, **Then** esos archivos se retiran de la raíz.
2. **Given** que uno de esos archivos contiene texto escrito por la persona, **When** se retira, **Then** se conserva una copia recuperable dentro del directorio de memoria del proyecto y la ruta de esa copia se informa en pantalla.
3. **Given** un proyecto con la copia obsoleta de la constitución en la raíz, **When** se instala o actualiza, **Then** esa copia se elimina.
4. **Given** una carpeta de configuración de agente que solo contiene el registro de gomemory, **When** se instala o actualiza, **Then** se retira por completo.
5. **Given** una carpeta de configuración de agente que además registra otros servidores ajenos a gomemory, **When** se instala o actualiza, **Then** solo se retira la entrada de gomemory y el resto del archivo queda intacto.
6. **Given** un archivo de configuración con formato corrupto o ilegible, **When** se instala o actualiza, **Then** no se modifica ni se borra, y se informa que se dejó sin tocar.
7. **Given** un proyecto sin ninguno de esos artefactos, **When** se instala o actualiza, **Then** la limpieza no informa nada ni falla.

---

### User Story 4 - Aplicar la constitución bajo demanda (Priority: P4)

Como persona que arranca una feature nueva, quiero pedir la constitución del proyecto con un comando y que se sirva desde la memoria, para trabajar siempre contra la versión vigente y editable, en vez de contra una copia congelada en el repositorio.

**Why this priority**: Cierra el ciclo de la constitución, pero es un flujo puntual y explícito; el resto de la feature funciona sin él.

**Independent Test**: Pedir la constitución y comprobar que devuelve el documento completo, y que tras editar la memoria devuelve la versión editada.

**Acceptance Scenarios**:

1. **Given** un proyecto con la semilla de constitución presente, **When** se pide la constitución, **Then** se devuelve el documento íntegro en un solo paso.
2. **Given** que la persona editó esa memoria, **When** se pide la constitución, **Then** se devuelve la versión editada, no la plantilla original.
3. **Given** un proyecto donde la semilla se borró, **When** se pide la constitución, **Then** se devuelve la plantilla de referencia y se avisa de que no proviene de la memoria del proyecto.
4. **Given** un proyecto que usa un flujo de especificación por fases, **When** se pide sincronizar la constitución, **Then** el documento del proyecto se actualiza con el contenido de la memoria.
5. **Given** un proyecto que NO usa ese flujo, **When** se pide sincronizar, **Then** no se crea ninguna estructura nueva y se informa que no aplica.
6. **Given** un agente compatible, **When** la persona invoca la constitución por su atajo nativo, **Then** el agente obtiene el contenido desde la memoria del proyecto y no desde una copia en el repositorio.

---

### User Story 5 - Reemplazar las reglas y la constitución con las mías (Priority: P2)

Como responsable de un equipo, quiero exportar las reglas de trabajo y la constitución
a un archivo, editarlas en mi editor y volver a importarlas —desde la consola o desde
la interfaz interactiva—, para que el contenido sea **el de mi equipo** y no el que
trae la herramienta por defecto.

**Why this priority**: sin esto, sembrar reglas y constitución convertiría a la
herramienta en la autora de las normas del equipo. Las plantillas que se envían son
un punto de partida, no doctrina: sin una vía de reemplazo cómoda, la memoria dejaría
de ser un contenedor neutral y pasaría a imponer un sesgo propio. Es P2 y no P3
porque es lo que hace legítimo el modelo de semillas de la Historia 1.

**Independent Test**: exportar un documento a un archivo, cambiarle una línea,
volver a importarlo y comprobar que el contexto del proyecto refleja el texto nuevo.

**Acceptance Scenarios**:

1. **Given** un proyecto con las semillas presentes, **When** exporto un documento fijado, **Then** recibo su contenido **vigente** —el que esté guardado ahora, no la plantilla original— en texto plano editable.
2. **Given** un archivo con mis propias reglas, **When** lo importo, **Then** el documento queda reemplazado y el contexto del proyecto pasa a emitir mi texto, con el mismo trato de documento fijado.
3. **Given** un documento que ya personalicé, **When** listo los documentos fijados, **Then** veo que está personalizado y cuándo se modificó por última vez, distinguible de uno que sigue con el contenido por defecto.
4. **Given** que me arrepiento de un cambio, **When** restauro el documento, **Then** vuelve al contenido por defecto de la herramienta.
5. **Given** que prefiero la interfaz interactiva, **When** entro a la sección de documentos fijados, **Then** puedo exportar, importar y restaurar con las mismas capacidades que por consola.
6. **Given** un archivo vacío o ilegible, **When** intento importarlo, **Then** la operación se rechaza con un motivo claro y el documento anterior queda intacto.
7. **Given** un documento fijado que no existe en el proyecto, **When** lo importo, **Then** se crea.

---

### Edge Cases

- **Semilla editada y luego versión nueva del binario**: la edición de la persona gana siempre; el binario nunca reescribe una semilla existente, aunque su plantilla haya cambiado.
- **Textos de plantilla no disponibles** en el binario en uso: la siembra se omite en silencio en lugar de crear memorias vacías que el agente tomaría por buenas.
- **Contexto en modo índice** (donde por contrato ningún cuerpo de memoria se emite): las reglas de trabajo también colapsan a su referencia, sin excepción — el modo índice sigue siendo un índice puro.
- **Presupuesto de contexto muy ajustado**: las reglas de trabajo se emiten completas de todos modos; son una excepción declarada al recorte, igual que los conflictos sin resolver.
- **Directorio de memoria sin permisos de escritura** al intentar respaldar un archivo antes de borrarlo: no se borra el archivo, se informa el error y la instalación continúa.
- **Archivos de instrucciones escritos íntegramente a mano por la persona** (nunca tocados por gomemory): se les aplica el mismo retiro con respaldo previo — es el comportamiento pedido de forma explícita, y el respaldo es la red de seguridad.
- **Instalación sobre un directorio que no es un proyecto de código** (sin control de versiones): el comportamiento no cambia; nada de esto depende del control de versiones.
- **Proyecto con cientos de memorias posteriores a la semilla**: las reglas de trabajo siguen apareciendo en el contexto. Si dependieran de la ventana de memorias recientes desaparecerían solas, sin error y sin aviso — es justo el modo de fallo que FR-031 cierra.
- **Lectura de la clave de tópico por la vía de las memorias recientes**: devuelve el valor real, no una cadena vacía. Antes de FR-030, cualquier comparación por clave sobre esa vía daba siempre falso.
- **Proyecto con la sincronización externa de documentación activada**: sembrar no publica nada fuera. Sin esta garantía, una instalación empujaría el documento entero de la constitución al sistema externo de la persona sin que nadie lo pidiera.
- **Plantilla de origen modificada en una versión futura** de forma que active un depurador de secretos: la siembra debe seguir guardando el texto íntegro; si alguna vez dejara de coincidir con el origen, debe detectarse, no pasar inadvertido.
- **Importar un archivo con exactamente el mismo contenido**: se acepta sin error y sin efecto observable; la operación es idempotente.
- **Importar sobre un documento y luego reinstalar**: la siembra no lo pisa (FR-003). El documento importado es tan propio de la persona como uno editado a mano.
- **Exportar a una ruta sin permisos de escritura**: falla con el motivo, sin dejar un archivo a medias y sin alterar la memoria.
- **Importar un archivo cuyo contenido queda vacío tras depurar secretos**: se rechaza como contenido vacío; nunca deja el documento en blanco.
- **Diagnóstico de activación**: dejar de escribir archivos de instrucciones de proyecto no debe reportarse como una falla en el informe de estado; debe reconocerse como "no aplica" con su motivo.

## Requirements *(mandatory)*

### Functional Requirements

#### Semillas de memoria

- **FR-001**: El sistema DEBE sembrar, la primera vez que se usa la memoria de un proyecto, una memoria con las reglas de trabajo, clasificada como preferencia y marcada con una clave de tópico estable y documentada.
- **FR-002**: El sistema DEBE sembrar, en el mismo momento, una memoria con la constitución del proyecto, clasificada como decisión de arquitectura y marcada con su propia clave de tópico estable.
- **FR-003**: El sistema NO DEBE modificar, reemplazar ni duplicar una memoria semilla que ya exista para ese proyecto, cualquiera sea su contenido actual.
- **FR-004**: El sistema DEBE sembrar tanto al instalar en un proyecto como al arrancar el servidor de memoria, de modo que quien nunca ejecuta la instalación por proyecto también obtenga las semillas.
- **FR-005**: El sistema DEBE omitir la siembra sin fallar ni informar error cuando el texto de una semilla no esté disponible en el binario en uso.
- **FR-006**: El sistema DEBE poder localizar una memoria por su clave de tópico sin depender de cuán reciente sea.

#### Corrección del defecto latente de la clave de tópico

> Estos dos requisitos no son andamiaje de las semillas: corrigen un defecto real
> que ya existe en el producto y que esta feature sería la primera en activar.

- **FR-030**: La consulta que lista las memorias recientes de un proyecto DEBE devolver la clave de tópico de cada memoria, igual que ya lo hace la consulta que lista todas. Hoy no la devuelve, de modo que toda persona o componente que lea la clave por esa vía la recibe vacía, sin error ni aviso.
- **FR-031**: La presencia de una memoria fijada en el contexto NO DEBE depender de la ventana de memorias recientes. El sistema DEBE resolverla por su clave de tópico, de forma que siga apareciendo por muchas memorias nuevas que se acumulen después.

#### Siembra inerte

> Guardar una memoria dispara varios efectos secundarios pensados para el trabajo
> diario (enlazar memorias co-activadas, reflejar decisiones en documentación
> externa, depurar secretos pegados por error). Ninguno tiene sentido sobre una
> semilla creada por la herramienta, y uno de ellos hoy causaría daño observable.

- **FR-032**: El texto guardado de una semilla DEBE ser idéntico, carácter por carácter, al texto de origen. Ningún mecanismo de depuración o anotación puede alterarlo.
- **FR-033**: Sembrar NO DEBE crear relaciones automáticas entre la semilla y otras memorias del proyecto. Una semilla no es fruto del trabajo de una sesión y no tiene co-activación que registrar.
- **FR-034**: Sembrar NO DEBE propagar la memoria a sistemas externos de documentación, **incluso cuando esa sincronización esté activada** por la persona. La constitución sembrada es material de referencia local, no una decisión de arquitectura recién tomada que merezca anunciarse fuera.

#### Reglas de trabajo en el contexto

- **FR-007**: El contexto del proyecto DEBE incluir las reglas de trabajo sembradas en una sección propia y claramente rotulada, ubicada antes de las secciones de memorias por tipo.
- **FR-008**: El contexto DEBE emitir el texto de las reglas de trabajo íntegro, sin aplicarle el recorte por presupuesto que se aplica al resto de memorias.
- **FR-009**: El contexto NO DEBE repetir las reglas de trabajo dentro de la sección general de preferencias.
- **FR-010**: En el modo índice del contexto, las reglas de trabajo DEBEN colapsar a su referencia como cualquier otra memoria.

#### Instalación sin artefactos

- **FR-011**: La instalación NO DEBE crear ni modificar archivos de instrucciones para agentes en la raíz del proyecto destino.
- **FR-012**: La instalación NO DEBE copiar el documento de la constitución a la raíz del proyecto destino.
- **FR-013**: La instalación NO DEBE crear carpetas de configuración para agentes que no fueron solicitados explícitamente; esos agentes DEBEN seguir siendo configurables mediante el comando de configuración de servidores que ya existe.
- **FR-014**: El mensaje final de la instalación DEBE indicar dónde quedaron las reglas y la constitución y cómo se aplican, y NO DEBE referirse a archivos que ya no se generan.
- **FR-015**: La actualización de la herramienta DEBE producir exactamente el mismo estado del proyecto que una instalación limpia.

#### Limpieza de artefactos previos

- **FR-016**: Al instalar o actualizar, el sistema DEBE retirar de la raíz los archivos de instrucciones para agentes que las versiones anteriores generaban o editaban.
- **FR-017**: Antes de retirar cada uno de esos archivos, el sistema DEBE guardar una copia íntegra dentro del directorio de memoria del proyecto e informar en pantalla la ruta de la copia.
- **FR-018**: Si la copia de respaldo no puede escribirse, el sistema NO DEBE retirar el archivo original y DEBE informar el motivo, sin interrumpir el resto de la instalación.
- **FR-019**: Al instalar o actualizar, el sistema DEBE eliminar la copia obsoleta del documento de constitución de la raíz, sin respaldarla, por ser recuperable desde la memoria y desde el propio binario.
- **FR-020**: Al instalar o actualizar, el sistema DEBE quitar el registro de gomemory de los archivos de configuración de agentes que la instalación creaba, y eliminar el archivo y su carpeta únicamente cuando no quede ningún otro registro ajeno a gomemory.
- **FR-021**: El sistema NO DEBE modificar un archivo de configuración cuyo contenido no pueda interpretarse, y DEBE informar que lo dejó intacto.
- **FR-022**: La limpieza DEBE ser idempotente y silenciosa cuando no hay nada que limpiar.
- **FR-023**: La limpieza NO DEBE tocar archivos de reglas que la instalación nunca creó por su cuenta.

#### Gestión de documentos fijados

> Las plantillas que se envían son un **punto de partida**, no doctrina. Sin una vía
> cómoda de reemplazo, sembrarlas convertiría a la herramienta en autora de las
> normas del equipo. Estos requisitos son los que mantienen la memoria como
> contenedor neutral.

- **FR-035**: El sistema DEBE mantener un catálogo de documentos fijados, cada uno con un alias estable y legible. Las capacidades de listar, exportar, importar y restaurar DEBEN ser agnósticas al número de documentos: añadir uno nuevo no puede exigir un comando ni una pantalla nuevos.
- **FR-036**: Los usuarios DEBEN poder exportar el contenido **vigente** de un documento fijado —el guardado en la memoria, no la plantilla de origen— en texto plano editable a mano.
- **FR-037**: Al exportar un documento que no existe en el proyecto, el sistema DEBE devolver el contenido por defecto y advertir que no proviene de la memoria.
- **FR-038**: Los usuarios DEBEN poder importar un documento desde un archivo. El documento importado DEBE conservar la identidad del documento fijado, de modo que siga recibiendo el mismo trato en el contexto del proyecto.
- **FR-039**: Importar un documento fijado que aún no existe DEBE crearlo.
- **FR-040**: El sistema DEBE rechazar la importación de contenido vacío o ilegible, indicando el motivo, y DEBE dejar el documento anterior **intacto**.
- **FR-041**: Listar, exportar, importar y restaurar DEBEN estar disponibles **tanto por consola como en la interfaz interactiva**, con las mismas capacidades en ambas.
- **FR-042**: La importación DEBE poder apuntar a cualquier documento identificado por su clave, incluso a uno que no figure en el catálogo. El catálogo es una comodidad, no un límite.
- **FR-043**: Los usuarios DEBEN poder restaurar un documento fijado a su contenido por defecto.
- **FR-044**: Al listar los documentos fijados, el sistema DEBE indicar para cada uno si conserva el contenido por defecto o fue personalizado, y cuándo se modificó por última vez.
- **FR-045**: La importación DEBE usar la misma vía inerte que la siembra: sin relaciones automáticas y sin publicación a sistemas externos (FR-033, FR-034).
- **FR-046**: El formato de estos documentos DEBE ser texto plano editable, distinto del volcado completo de la memoria que ya existe para mover memorias entre proyectos. Ninguna de las dos capacidades reemplaza a la otra y ambas DEBEN seguir disponibles.

#### Constitución bajo demanda

- **FR-024**: Los usuarios DEBEN poder obtener la constitución vigente del proyecto con una sola invocación, servida desde la memoria.
- **FR-025**: Cuando la memoria de constitución no exista, el sistema DEBE devolver la plantilla de referencia y advertir explícitamente que no proviene de la memoria del proyecto.
- **FR-026**: Los usuarios DEBEN poder sincronizar el documento de constitución del proyecto con el contenido de la memoria, y esa sincronización NO DEBE crear estructura alguna en proyectos que no usen ese flujo.
- **FR-027**: El sistema DEBE distribuir un atajo nativo de constitución para los agentes que lo soportan; ese atajo DEBE resolver el contenido desde la memoria en el momento de la invocación, NO DEBE incluir una copia del texto, y su ausencia o fallo NO DEBE interrumpir la instalación.

#### Diagnóstico

- **FR-028**: El informe de estado de activación DEBE reportar el canal de instrucciones de ámbito de proyecto como "no aplica", con el motivo declarado, en lugar de como una falla.
- **FR-029**: El informe DEBE seguir evaluando normalmente el canal de instrucciones de ámbito de usuario, que sí se mantiene.

### Key Entities

- **Memoria semilla**: una memoria del proyecto creada automáticamente la primera vez que se usa la memoria. Se distingue de cualquier otra por una clave de tópico estable y conocida. Es propiedad de la persona desde el momento en que existe: el sistema la crea, nunca la corrige.
- **Reglas de trabajo**: la semilla que describe cómo debe trabajar el agente (cuándo planificar, cómo verificar, cómo tratar los bugs). Se clasifica como preferencia y goza de emisión íntegra en el contexto.
- **Constitución**: la semilla que describe cómo debe escribirse el código (capas, stack, convenciones). Se clasifica como decisión de arquitectura y se consulta bajo demanda, no en cada sesión.
- **Documento fijado**: una memoria semilla vista desde la perspectiva de quien la administra — con alias legible, contenido exportable e importable, y un estado observable (por defecto o personalizada). El catálogo de documentos fijados es la lista de los que la herramienta conoce por su nombre.
- **Artefacto legado**: cualquier archivo o carpeta que versiones anteriores de la instalación creaban en el proyecto destino y que esta feature retira.
- **Respaldo de archivos de agente**: la copia que se conserva dentro del directorio de memoria del proyecto antes de retirar un archivo de instrucciones.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tras instalar en un directorio de proyecto vacío, la raíz contiene 0 archivos de instrucciones, 0 copias de la constitución y 0 carpetas de configuración de agentes no solicitados.
- **SC-002**: En el 100% de los inicios de sesión de un proyecto con las semillas presentes, el agente recibe el texto completo de las reglas de trabajo sin ejecutar ninguna acción adicional.
- **SC-003**: Tras 5 instalaciones consecutivas sobre un proyecto cuya memoria de reglas fue editada, el contenido editado se conserva sin un solo cambio.
- **SC-004**: En un proyecto que arrastra artefactos de una instalación anterior, tras una sola actualización quedan 0 artefactos legados y el 100% del texto propio de los archivos retirados es recuperable desde el respaldo.
- **SC-005**: El texto del protocolo de memoria deja de estar duplicado entre el archivo de instrucciones y lo que el agente ya recibe al conectarse: la medición de uso de contexto muestra una única copia por sesión, no dos.
- **SC-006**: Obtener la constitución vigente toma una sola invocación y devuelve el documento completo.
- **SC-007**: El informe de estado de activación reporta 0 fallas atribuibles a la ausencia de archivos de instrucciones de proyecto.
- **SC-008**: La limpieza no destruye ningún registro de configuración ajeno a gomemory: en un proyecto con otros servidores registrados, el 100% de esos registros sobrevive.
- **SC-009**: En un proyecto con 200 memorias creadas después de la semilla, las reglas de trabajo siguen apareciendo íntegras en el contexto: 0 desapariciones silenciosas.
- **SC-010**: Al listar memorias recientes, el 100% de las que tienen clave de tópico la reportan con su valor real; hoy la reportan vacía en el 100% de los casos.
- **SC-011**: Sembrar produce 0 relaciones automáticas nuevas, 0 publicaciones a sistemas externos (aun con la sincronización activada) y 0 diferencias de texto respecto al origen.
- **SC-012**: Reemplazar las reglas del equipo toma 3 pasos: exportar, editar, importar — sin tocar la base de datos a mano ni conocer ninguna clave interna.
- **SC-013**: Las cuatro operaciones sobre documentos fijados (listar, exportar, importar, restaurar) están disponibles al 100% por consola y al 100% en la interfaz interactiva.
- **SC-014**: Un documento importado sobrevive al 100% de las reinstalaciones y actualizaciones posteriores.
- **SC-015**: Añadir un documento fijado nuevo al catálogo no requiere ningún comando ni pantalla nueva: 0 archivos de interfaz modificados.

## Assumptions

- El agente recibe el protocolo de memoria al conectarse al servidor de memoria; por eso repetirlo en un archivo del repositorio es redundante y no una segunda red de seguridad. Los agentes que no se conectan por ese canal quedan cubiertos por el ámbito de usuario, que esta feature no modifica.
- Retirar por completo los archivos de instrucciones del proyecto —incluido su contenido propio— es una decisión ya tomada y confirmada. El respaldo previo es una red de seguridad local, no un cambio del comportamiento pedido.
- Los textos de las reglas de trabajo y de la constitución ya existen dentro del producto; esta feature los reubica, no los redacta.
- El presupuesto de contexto vigente recorta las memorias largas, por lo que emitir la constitución íntegra en cada sesión sería contraproducente: se consulta bajo demanda a propósito.
- Los agentes que hoy se configuran automáticamente y dejan de configurarse siguen siendo alcanzables mediante el comando explícito de configuración de servidores; no se pierde soporte, se pierde la creación no solicitada.
- Retirar un canal de activación por proyecto no degrada la cobertura, porque el mismo contenido llega por el canal de conexión y por el ámbito de usuario.
- Los archivos de instrucciones de ámbito de usuario (fuera del proyecto) están fuera del alcance de esta feature.
- Las plantillas de reglas y constitución que se envían con la herramienta son un punto de partida razonable, no una norma que el producto imponga. La capacidad de reemplazarlas es lo que mantiene la memoria como contenedor neutral en vez de convertirla en autora de las normas del equipo.
- El volcado completo de memorias en formato de intercambio y la exportación de un documento fijado en texto plano resuelven necesidades distintas —mover la memoria entre proyectos frente a editar un documento a mano— y conviven sin solaparse.
