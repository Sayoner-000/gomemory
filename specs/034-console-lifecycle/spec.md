# Feature Specification: Ciclo de vida de gomemory en consola — instalar, actualizar y desinstalar sin rastros

**Feature Branch**: `034-console-lifecycle`

**Created**: 2026-09-26

**Status**: Draft

**Input**: User description: "el mem install cada vez que se hace por un directorio crea el binario 'mem' en ese proyecto y no lo hace global, lo cual deja que esos binarios no estén con la línea base actualizada del proyecto, y ¿cómo podemos hacer que se envíe un heartbeat de que hay una versión disponible y que se actualice, como lo hacen otros proyectos como Codex?". Ampliaciones de la persona: «cuando se instale gomemory o se actualice, que realice el mismo mecanismo en consola que https://www.skills.sh/» y «agrega también en el mecanismo interactivo la desinstalación, y que no queden rastros de gomemory en el sistema si el usuario desea quitarlo». Decisiones tomadas: con un binario global no se copia al proyecto y las copias viejas se retiran; ante una versión nueva **solo se avisa** (estilo Codex), sin autoactualización.

## Contexto del problema *(no normativo — fundamenta el porqué)*

Evidencia medida en la máquina de la persona el 2026-09-26:

- `mem install` copia el binario que se ejecuta a `<proyecto>/mem`. El binario global (`~/.local/bin/mem`) está en **2.26.4**, pero las copias por proyecto se han quedado en **2.8.0** (certified_kolmena, kong_gateway, themes), **2.9.0** (test), **2.10.1** (lab) y **2.16.9** (mcp_mediator_core).
- Hooks y MCP ya usan el binario global cuando está en el PATH. Aun así, la copia local se sigue usando en cuatro casos:
  - el protocolo de memoria recomienda `./mem …` como alternativa por CLI;
  - el script del brazo extensor de spec-kit busca primero `./mem`;
  - los hooks recurren a la copia local si no hay binario global;
  - `./mem update` actualiza **solo la copia local** y deja el global como estaba.
- No existe ningún aviso de versión nueva. Hay que ejecutar `mem update --check` a mano.
- `mem update` no verifica el `checksums.txt` que acompaña a cada release.
- `mem install` configura siempre OpenCode, Claude Code, Cursor y Codex, estén instalados o no.
- `mem uninstall` anuncia que borrará "TODA la memoria guardada", pero solo borra `<proyecto>/.memory`. Desde la spec 005 la base de memoria vive en el almacén global del usuario, y **se queda en la máquina**. Tampoco retira la configuración global de los agentes, ni el binario global, ni el almacén global.
- Hoy el almacén global no sabe en qué ruta está cada proyecto: identifica cada uno con una clave derivada.

Referencia de experiencia de consola: `npx skills add` (skills.sh) detecta los agentes instalados y los ofrece en selección múltiple, pregunta el alcance (proyecto o global), propone una opción recomendada, muestra un resumen y pide confirmación. Todo tiene un flag no interactivo equivalente (`-y`, `-g`, `-a`). Referencia de aviso de versión: Codex CLI consulta el último release como mucho una vez al día, guarda el resultado en caché local y muestra un aviso con el comando para actualizar. No se actualiza en silencio.

## Clarifications

### Session 2026-09-26

- Q: ¿Cómo encuentra la desinstalación de sistema los proyectos anteriores al registro de rutas? → A: `--all` escanea por defecto el directorio personal con una profundidad máxima de 6 niveles y omite directorios pesados; `--scan <dir>` cambia la raíz y `--no-scan` desactiva el escaneo.
- Q: ¿Dónde y cómo se exporta la memoria antes de borrarla? → A: por defecto en `~/gomemory-export-AAAAMMDD-HHMMSS/`, un archivo de `mem export` por proyecto más un índice (proyecto → ruta original); `--export <dir>` cambia el destino.
- Q: ¿La retirada de copias locales se hace en silencio o avisando? → A: se retira automáticamente y se muestra una sola vez a la persona un aviso visible con la ruta, la versión retirada y la versión global en uso.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Un solo binario, siempre al día (Priority: P1)

Una persona trabaja en varios proyectos con gomemory. Quiere que todos usen la misma versión, la que tiene instalada globalmente, y que actualizar una vez baste para todos, sin copias olvidadas que se queden atrás.

**Why this priority**: es la causa raíz del problema que se ha reportado. Mientras existan copias por proyecto, cualquier corrección (incluidas las de seguridad de v2.26.3 y v2.26.4) no llega a esos proyectos. Todo lo demás (aviso de versión, desinstalación completa) depende de que haya una única fuente de verdad.

**Independent Test**: en una máquina con un binario global y un proyecto con una copia local antigua, abrir una sesión de agente en ese proyecto (o ejecutar `mem install`) y comprobar que la copia local desaparece y que todo sigue funcionando con el global.

**Acceptance Scenarios**:

1. **Given** hay un binario global en el PATH, **When** la persona ejecuta `mem install` en un proyecto nuevo, **Then** no se crea ningún binario dentro del proyecto y la instalación informa de que usa el global.
2. **Given** un proyecto tiene `<proyecto>/mem` en la versión 2.8.0 y el global está en 2.26.4, **When** se inicia una sesión de agente en ese proyecto, **Then** la copia local se retira, la sesión funciona sin errores y la persona ve una sola vez el aviso "Se retiró `<proyecto>/mem` (v2.8.0); ahora se usa el global v2.26.4".
3. **Given** `<proyecto>/mem` es un archivo que no es un binario de gomemory (por ejemplo, un script propio de la persona), **When** se ejecuta la retirada de copias, **Then** el archivo no se toca.
4. **Given** la persona ejecuta `./mem update` desde una copia local y existe un global, **When** termina la actualización, **Then** el global queda en la versión nueva y la copia local se retira.
5. **Given** no hay binario global en el PATH, **When** se ejecuta `mem install`, **Then** se mantiene la copia en el proyecto como alternativa (comportamiento actual) y se explica cómo pasar a una instalación global.
6. **Given** hay un global disponible, **When** el agente lee el protocolo de memoria o se ejecuta el brazo extensor de spec-kit, **Then** ambos usan `mem …` y no `./mem …`.

---

### User Story 2 - Desinstalar sin dejar rastros (Priority: P2)

Una persona quiere quitar gomemory de un proyecto concreto, o de todo su sistema. Espera que, al terminar, no quede ni el binario, ni la memoria (salvo que decida conservarla o exportarla), ni ninguna entrada en la configuración de sus agentes, y que la configuración de otras herramientas que comparte esos archivos siga intacta.

**Why this priority**: hoy la desinstalación deja la memoria completa en el disco mientras promete lo contrario. Eso es un problema de privacidad y de confianza. Además, la persona no tiene forma de salir del sistema de manera limpia.

**Independent Test**: en un entorno aislado (directorio personal y almacén temporales), instalar gomemory en dos proyectos, ejecutar la desinstalación de sistema sin interacción y comprobar que no queda ninguna ruta ni entrada de gomemory, y que las entradas de otras herramientas en los archivos compartidos siguen igual.

**Acceptance Scenarios**:

1. **Given** una terminal interactiva, **When** la persona ejecuta `mem uninstall`, **Then** puede elegir entre "Solo este proyecto" y "Todo gomemory del sistema", ve un inventario de **lo que existe realmente** agrupado por categorías (binario, memoria con su tamaño, configuración de agentes y archivos del proyecto) y elige qué hacer con la memoria: exportar antes de borrar (recomendado), borrar sin exportar o conservarla.
2. **Given** se elige el alcance de sistema, **When** se llega a la confirmación, **Then** hay que escribir `gomemory` para continuar. Cualquier otra entrada cancela sin cambiar nada.
3. **Given** se elige "Solo este proyecto" y "borrar", **When** termina, **Then** también ha desaparecido la base de memoria de ese proyecto en el almacén global (corrige el defecto actual).
4. **Given** `~/.claude.json` o `~/.codex/config.toml` contienen entradas de otras herramientas además de las de gomemory, **When** termina la desinstalación de sistema, **Then** solo se han retirado las entradas de gomemory y el resto del archivo sigue igual.
5. **Given** falla uno de los pasos (por ejemplo, por falta de permisos sobre un archivo), **When** termina la desinstalación, **Then** los demás pasos se han completado y el resumen marca ese elemento con ⚠, la ruta y el comando para quitarlo a mano.
6. **Given** se usa `--dry-run`, **When** termina, **Then** se muestra el inventario completo y no se ha modificado ningún archivo.
7. **Given** no hay terminal interactiva y no se pasó `--yes`, **When** se ejecuta `mem uninstall`, **Then** no se borra nada y el comando termina con un código de error.
8. **Given** hay proyectos instalados antes de que existiera el registro de rutas dentro del directorio personal, **When** la persona ejecuta la desinstalación de sistema sin opciones de escaneo, **Then** esos proyectos se detectan con el escaneo por defecto y se incluyen en el inventario. Con `--scan <dir>` se detectan los que estén bajo esa otra raíz, y con `--no-scan` solo se incluyen los registrados.

---

### User Story 3 - Instalar y actualizar con una consola guiada (Priority: P3)

Una persona instala o actualiza gomemory desde la terminal y quiere que la guíen: que le muestre qué agentes tiene instalados, que le recomiende la mejor opción, que resuma lo que va a hacer antes de hacerlo y que al final le diga qué salió bien y qué no. En CI, en scripts o cuando lo lanza otro proceso, todo debe funcionar sin preguntar.

**Why this priority**: mejora la experiencia y evita configurar agentes que la persona no usa, pero el sistema ya funciona sin ello. Depende de P1 (dónde vive el binario) y la reutiliza.

**Independent Test**: ejecutar `mem install` en una terminal real y comprobar la secuencia de pasos. Después, ejecutar `mem install --yes` y `mem install` con la entrada redirigida, y comprobar que ambos terminan sin esperar respuesta.

**Acceptance Scenarios**:

1. **Given** una terminal interactiva con Claude Code y Codex instalados, pero no OpenCode ni Cursor, **When** la persona ejecuta `mem install`, **Then** ve una selección múltiple de agentes con Claude Code y Codex ya marcados, elige el alcance de la configuración (proyecto o global), ve un resumen y confirma antes de que se escriba nada.
2. **Given** no hay binario global, **When** se ejecuta `mem install` de forma interactiva, **Then** se ofrece "Global (Recomendado)" o "Copia en el proyecto", explicando la diferencia.
3. **Given** la persona eligió solo Claude Code, **When** termina la instalación, **Then** no se ha escrito configuración para los agentes no elegidos, y el resumen final muestra ✓ o ⚠ por cada paso.
4. **Given** se usa `--yes`, o no hay terminal interactiva, **When** se ejecuta `mem install`, **Then** se aplican los valores recomendados (o la selección guardada) y el comando termina sin esperar entrada.
5. **Given** un proyecto instalado con una selección de agentes, **When** se ejecuta `mem update`, **Then** la reinstalación del proyecto reutiliza esa selección sin preguntar.
6. **Given** hay una versión nueva, **When** la persona ejecuta `mem update` en una terminal interactiva, **Then** ve "Actual vX → Disponible vY", confirma y sigue el progreso de cada paso (descarga, verificación de integridad, sustitución del binario, refresco del proyecto y retirada de copias) hasta un resumen final.
7. **Given** la persona instala gomemory con el instalador de consola en una terminal interactiva, **When** el instalador termina dentro de un proyecto, **Then** ofrece lanzar `mem install` guiado en ese proyecto.

---

### User Story 4 - Enterarse de que hay una versión nueva (Priority: P4)

Una persona usa gomemory a diario a través de su agente y nunca ejecuta comandos de mantenimiento. Quiere enterarse cuando sale una versión nueva, con el comando exacto para actualizar, sin que eso ralentice sus sesiones y pudiendo desactivarlo. Cuando actualice, quiere la garantía de que el binario descargado es el publicado.

**Why this priority**: sin un aviso, la persona no sabe que existen v2.26.3 y v2.26.4 (correcciones de seguridad). Tiene menos prioridad porque requiere que P1 funcione para que la actualización llegue a todos los proyectos.

**Independent Test**: con una caché de versiones que indica una versión mayor que la instalada, iniciar una sesión de agente y comprobar que la persona ve el aviso una sola vez. Con el aviso desactivado, comprobar que no hay tráfico de red.

**Acceptance Scenarios**:

1. **Given** la última consulta de versión tiene más de 24 horas, **When** se inicia una sesión de agente, **Then** la consulta se lanza en segundo plano y el arranque de la sesión no espera a la red.
2. **Given** la caché indica que la versión publicada es mayor que la instalada, **When** se inicia una sesión, **Then** la persona ve "gomemory vY disponible (tienes vX) → mem update", como mucho una vez por versión y sesión.
3. **Given** el aviso está desactivado por ajuste o por variable de entorno, **When** se inician sesiones, **Then** no se hace ninguna consulta de red ni se muestra ningún aviso.
4. **Given** el archivo descargado no coincide con su suma de verificación publicada, **When** se ejecuta `mem update`, **Then** la actualización se aborta, el binario instalado no se toca y se informa del motivo.
5. **Given** la persona ejecuta `mem doctor`, **When** termina, **Then** ve la versión del binario global, las copias locales detectadas con su versión y el estado de la consulta de versiones.

---

### Edge Cases

- La consulta de versión falla (sin red, límite de peticiones, respuesta inválida): no se muestra ningún aviso, no se rompe la sesión, se conserva la caché anterior y se reintenta pasado el intervalo.
- Varias sesiones arrancan a la vez y todas ven la caché caducada: pueden lanzarse varias consultas, pero la caché nunca queda corrupta (escritura atómica) y el aviso sigue mostrándose una vez por versión y sesión.
- La copia local está en uso (otro proceso la ejecuta) cuando se intenta retirar: en Unix se retira igualmente; en Windows se reintenta en la siguiente ocasión, sin error visible.
- El binario global no admite escritura para la persona (por ejemplo, está en `/usr/local/bin` sin permisos): `mem update` no falla a medias. Informa del comando exacto para actualizarlo con los permisos adecuados.
- La persona desinstala el sistema desde una copia local, no desde el global: se retiran ambos.
- Hay proyectos sin registrar fuera del directorio personal, o más profundos de 6 niveles: el escaneo por defecto no los encuentra, y el resumen final lo advierte con el comando `--scan <dir>` para completarlo.
- El escaneo encuentra un directorio sin permiso de lectura: lo omite, sigue con el resto y lo cita en el resumen.
- Un proyecto registrado ya no existe en el disco: la desinstalación de sistema lo omite y lo indica en el resumen, y sus datos en el almacén global se retiran igualmente.
- Un archivo de configuración compartido tiene un formato que no se puede interpretar: no se modifica, y se marca con ⚠ y la instrucción manual.
- La exportación previa a borrar falla: la memoria de ese proyecto **no** se borra y se informa.
- `--keep-memory` junto con el alcance de sistema: se retira todo salvo las bases de memoria y la persona recibe la ruta donde quedan.
- La terminal es interactiva pero demasiado estrecha, o no admite color: la consola se degrada a texto plano sin perder información.
- Un hook o un subproceso ejecutan `mem install` en un proyecto sin selección guardada: se aplican los agentes detectados, sin preguntar.
- El `mem` del proyecto es un binario de gomemory para otra plataforma (por ejemplo, Linux dentro de un contenedor): como no se puede ejecutar `version`, no se identifica como copia (FR-003) y se conserva, tanto en la retirada automática como en la desinstalación de sistema. Validado el 2026-09-26 en la máquina real (T082).
- La desinstalación de sistema conserva `[features] hooks = true` en `~/.codex/config.toml`: es una bandera de Codex compartida con otras herramientas, no una entrada de gomemory (FR-012).

## Requirements *(mandatory)*

### Functional Requirements

**Binario único (P1)**

- **FR-001**: Si existe un binario global de gomemory accesible por el PATH y distinto del archivo del proyecto, la instalación MUST NOT copiar el binario al proyecto.
- **FR-002**: Si no existe un binario global, la instalación MUST mantener la copia en el proyecto como alternativa e indicar cómo pasar a una instalación global.
- **FR-003**: La instalación y el inicio de sesión de agente MUST retirar `<proyecto>/mem` (o su equivalente en Windows) cuando existe un global y la copia se identifica como binario de gomemory y no es el mismo archivo que el global. Cualquier otro archivo con ese nombre MUST NOT tocarse.
- **FR-004**: La retirada de copias MUST ser best-effort y no bloqueante: un fallo no puede retrasar ni interrumpir el inicio de sesión.
- **FR-004a**: Cada retirada MUST comunicarse a la persona una sola vez, mediante un aviso visible (no solo en el contexto del modelo) que indique la ruta retirada, la versión que tenía y la versión global que la sustituye. En `mem install` el aviso forma parte del resumen final.
- **FR-005**: El protocolo de memoria que se escribe en los archivos de instrucciones de los agentes y el script del brazo extensor de spec-kit MUST referirse al binario como `mem` (el del PATH) cuando hay un global, sin preferir una copia local.
- **FR-006**: Si `mem update` se ejecuta desde una copia local y existe un global, MUST actualizar el global y retirar la copia. Si el global no admite escritura, MUST informar del comando exacto para hacerlo.

**Desinstalación (P2)**

- **FR-007**: `mem uninstall` MUST ofrecer dos alcances: solo el proyecto actual, o todo gomemory en el sistema.
- **FR-008**: Antes de borrar nada, MUST presentar un inventario de los elementos que existen realmente, agrupados en binario, memoria (con su tamaño), configuración de agentes y archivos del proyecto.
- **FR-009**: MUST ofrecer tres opciones para la memoria: exportar antes de borrar (recomendada), borrar sin exportar o conservar. Si la exportación falla, la memoria afectada MUST NOT borrarse.
- **FR-009a**: La exportación MUST producir, por defecto en `~/gomemory-export-AAAAMMDD-HHMMSS/`:
  - un archivo por proyecto, en el formato de `mem export`, que se pueda reimportar por separado con `mem import`;
  - un índice que relacione cada archivo con su proyecto y su ruta original.
  `--export <dir>` MUST cambiar el destino. El destino MUST quedar fuera del almacén global, que se va a borrar, y MUST crearse con permisos privados del propietario (directorio 0700, archivos 0600). El resumen final MUST mostrar la ruta de la exportación.
- **FR-010**: En el alcance de proyecto con la memoria borrada, MUST eliminarse también la base de memoria del proyecto en el almacén global.
- **FR-011**: En el alcance de sistema, MUST retirarse:
  - el binario global;
  - el almacén global completo (bases de memoria, copias de seguridad y caché de versiones);
  - las entradas de gomemory en la configuración global de Claude Code, OpenCode, Codex, Cursor, Windsurf y Cline;
  - las skills y los prompts globales que instaló gomemory;
  - la integración de cada proyecto conocido.
- **FR-012**: En los archivos de configuración compartidos con otras herramientas MUST retirarse solo las entradas de gomemory. El archivo MUST NOT borrarse entero ni perder entradas ajenas.
- **FR-013**: La instalación MUST registrar la ruta de cada proyecto en el almacén global, para que la desinstalación de sistema pueda encontrarlo. Para detectar las instalaciones anteriores al registro, la desinstalación de sistema MUST escanear por defecto el directorio personal, con una profundidad máxima de 6 niveles y sin entrar en directorios pesados o de dependencias (control de versiones, dependencias de paquetes, *vendor* y cachés). Un directorio cuenta como proyecto de gomemory solo si contiene la configuración de gomemory (`.memory/settings.json`). El escaneo MUST NOT seguir enlaces simbólicos.
- **FR-014**: La desinstalación de sistema MUST seguir este orden: proyectos, configuración global de agentes, almacén global y, por último, el binario propio. En Windows, el binario en ejecución MUST retirarse de forma diferida.
- **FR-015**: Un paso fallido MUST NOT abortar los siguientes. El resumen final MUST mostrar cada elemento como ✓ o ⚠, con la ruta y el comando manual en el caso ⚠.
- **FR-016**: La confirmación del alcance de sistema MUST exigir que se escriba `gomemory`.
- **FR-017**: MUST existir los modos no interactivos `--all`, `--yes`, `--keep-memory`, `--export <dir>`, `--scan <dir>` (cambia la raíz del escaneo por defecto), `--no-scan` (solo proyectos registrados) y `--dry-run`. `--dry-run` MUST NOT modificar ningún archivo.
- **FR-018**: Sin terminal interactiva y sin `--yes`, la desinstalación MUST NOT borrar nada y MUST terminar con un código de error distinto de cero.
- **FR-019**: El mensaje de confirmación MUST describir con exactitud lo que se va a borrar. Ningún texto puede prometer más, ni menos, de lo que se hace.

**Consola guiada de instalación y actualización (P3)**

- **FR-020**: Con una terminal interactiva y sin `--yes`, `mem install` MUST presentar, en este orden:
  1. los agentes compatibles en selección múltiple, con los detectados en la máquina ya marcados;
  2. la ubicación del binario (global, recomendada, o copia en el proyecto), solo si no hay un global;
  3. el alcance de la configuración de agentes (proyecto o global);
  4. un resumen con confirmación antes de escribir nada.
- **FR-021**: La instalación MUST configurar solo los agentes seleccionados.
- **FR-022**: MUST existir flags equivalentes para cada decisión (`--yes`/`-y`, `--agents <lista>`, `--scope project|global`). Sin terminal interactiva, la instalación MUST comportarse como `--yes` y nunca esperar entrada.
- **FR-022a**: El alcance `global` MUST admitir solo Claude Code, Codex y OpenCode; una selección que incluya otro agente MUST rechazarse antes de escribir archivos. En ese alcance, la instalación MUST configurar únicamente las integraciones globales de los agentes seleccionados y MUST NOT crear configuración ni hooks de agente en el proyecto actual.
- **FR-023**: La selección de agentes y el alcance MUST guardarse en la configuración del proyecto, incluida una selección explícitamente vacía. Las reinstalaciones posteriores, incluida la que lanza `mem update`, MUST reutilizarlos sin preguntar ni volver a detectar agentes cuando la selección guardada está vacía; una instalación interactiva posterior MUST mostrar la selección guardada.
- **FR-024**: Instalación, actualización y desinstalación MUST terminar con un resumen de ✓ o ⚠ por paso.
- **FR-025**: `mem update` MUST mostrar la versión actual y la disponible, pedir confirmación en una terminal interactiva (salvo con `--yes`) e informar del progreso de cada paso.
- **FR-026**: Los instaladores de consola (Unix y Windows) MUST ofrecer, al terminar y si hay terminal interactiva, lanzar la instalación guiada en el directorio actual cuando es un proyecto. Sin terminal interactiva, MUST limitarse a mostrar el siguiente paso.
- **FR-027**: La consola MUST degradarse a texto plano sin perder información cuando la terminal no admite color o no tiene el ancho suficiente.

**Aviso de versión e integridad (P4)**

- **FR-028**: El sistema MUST consultar la última versión publicada como mucho una vez cada 24 horas y guardar el resultado en una caché del almacén global (versión, momento de la consulta y validador de caché del servidor).
- **FR-029**: La consulta MUST ejecutarse fuera del camino crítico del inicio de sesión, en segundo plano. La escritura de la caché MUST ser atómica.
- **FR-030**: Si la versión publicada es mayor que la instalada, el inicio de sesión MUST mostrar a **la persona** (no solo al modelo) el aviso "gomemory vY disponible (tienes vX) → mem update", como mucho una vez por versión y sesión.
- **FR-031**: El aviso y la consulta MUST poder desactivarse con un ajuste del proyecto y con una variable de entorno. Desactivados, MUST NOT producirse llamadas de red. La variable MUST documentarse junto al resto de la configuración por entorno.
- **FR-032**: `mem update` MUST verificar la suma de verificación publicada del archivo descargado antes de sustituir el binario. Si no coincide o falta, MUST abortar sin modificar el binario instalado.
- **FR-033**: `mem doctor` MUST informar de la versión del binario global, las copias locales detectadas con su versión y el estado de la caché de versiones (fecha de la última consulta y resultado).
- **FR-034**: El sistema MUST NOT actualizarse de forma automática ni enviar telemetría.

**Transversales**

- **FR-035**: Instalación, retirada de copias y desinstalación MUST ser idempotentes: repetirlas sobre el mismo estado no produce cambios adicionales ni errores.

### Key Entities *(include if feature involves data)*

- **Binario global**: el ejecutable de gomemory accesible por el PATH. Es la única fuente de verdad de la versión.
- **Copia local**: un ejecutable de gomemory dentro de un proyecto. Solo es legítima si no hay binario global.
- **Registro de proyecto**: la asociación entre un proyecto del almacén global y su ruta en el disco. Permite desinstalar todos los proyectos.
- **Selección de instalación**: agentes elegidos y alcance de la configuración de un proyecto. Persiste y la reutilizan las reinstalaciones.
- **Caché de versiones**: la última versión publicada conocida, cuándo se consultó y el validador del servidor. Es global al usuario.
- **Inventario de desinstalación**: la lista de elementos existentes, con su categoría, ruta, tamaño y resultado (✓/⚠) tras la ejecución.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tras actualizar el binario global y abrir una vez cada proyecto, el 100 % de los proyectos con copia local ya no la tienen. En la máquina de referencia, las 6 copias con versiones 2.8.0–2.16.9 desaparecen.
- **SC-002**: Una sola actualización deja todos los proyectos en la misma versión, sin pasos manuales por proyecto.
- **SC-003**: En un entorno aislado con dos proyectos instalados, la desinstalación de sistema deja **0** archivos, directorios y entradas de configuración de gomemory, y **0** cambios en las entradas de otras herramientas.
- **SC-004**: La simulación (`--dry-run`) produce 0 modificaciones en el sistema de archivos.
- **SC-005**: La instalación sin interacción (`--yes` o sin terminal) termina sin esperar entrada en el 100 % de las ejecuciones, incluida la que lanza la actualización.
- **SC-006**: El inicio de sesión de agente no tarda más, de forma apreciable, por la consulta de versión: la consulta nunca bloquea el arranque, tenga o no red.
- **SC-007**: Cuando hay una versión nueva, la persona la ve en la primera sesión posterior a la consulta, y como mucho una vez por versión y sesión.
- **SC-008**: Con el aviso desactivado, se producen 0 peticiones de red relacionadas con versiones.
- **SC-009**: El 100 % de los archivos descargados que no coinciden con su suma de verificación se rechazan sin alterar el binario instalado.
- **SC-010**: Una persona que instala gomemory por primera vez completa la instalación guiada, con sus agentes correctos, en menos de 1 minuto y sin consultar documentación.
- **SC-011**: El 100 % de los proyectos exportados antes de una desinstalación se pueden reimportar en una instalación limpia con el mismo número de memorias que tenían.

## Assumptions

- Los agentes que se ofrecen son los que gomemory ya sabe configurar (Claude Code, Codex, OpenCode y Cursor en la instalación; Windsurf y Cline como opcionales). No se añaden agentes nuevos.
- Un agente se considera "detectado" si su ejecutable está en el PATH o existe su directorio de configuración en el directorio personal. Sin selección guardada ni agentes detectados (un directorio personal vacío, CI), se configuran los que `install` configuraba siempre antes de esta feature (Claude Code, OpenCode, Cursor y Codex), para no dejar sin integración un entorno sin señales (decisión de implementación, 2026-09-26).
- La ubicación global recomendada del binario es la misma que ya usa el instalador de consola (`~/.local/bin`, o `/usr/local/bin` si admite escritura).
- El intervalo de consulta es de 24 horas, como en Codex. La fuente de la versión publicada es la misma que ya usa `mem update`.
- El formato de exportación de la memoria es el que ya produce `mem export`, y se puede reimportar con `mem import`.
- El registro de rutas solo cubre los proyectos que se instalen o reinstalen después de esta feature. Para los anteriores, la desinstalación de sistema usa el escaneo por defecto (FR-013), y la actualización (que reinstala el proyecto actual) los va registrando al usarlos.
- La consola interactiva usa las librerías de interfaz de terminal que el proyecto ya incluye. No se añaden dependencias.
- Quedan fuera de alcance la actualización automática, la telemetría y descubrir proyectos sin registrar fuera de la raíz y la profundidad del escaneo.
