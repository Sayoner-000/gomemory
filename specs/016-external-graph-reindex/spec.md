# Feature Specification: Reindexado dual de grafos de código + edición de huella de contexto en TUI

**Feature Branch**: `016-external-graph-reindex`

**Created**: 2026-08-15

**Status**: Draft

**Input**: User description: "mem index refresca ambos grafos + acciones equivalentes en la TUI"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Un solo comando refresca ambos grafos de código (Priority: P1)

Como usuario de `mem` en la línea de comandos, al ejecutar el comando de indexado quiero que se actualice tanto el grafo de código propio (que solo cubre archivos Go) como el grafo de código externo multi-lenguaje (cuando el proveedor opcional está instalado), sin tener que recordar invocar una segunda herramienta por separado.

**Why this priority**: Es el hueco de flujo de trabajo más costoso hoy: el grafo externo se queda desactualizado porque actualizarlo requiere un paso manual adicional que es fácil de olvidar. Resolver esto entrega valor inmediato con el menor cambio de superficie.

**Independent Test**: Puede probarse completamente ejecutando el comando de indexado en un proyecto con el proveedor externo instalado y verificando que, al terminar, ambos grafos (el propio y el externo) reflejan el estado actual del código, sin ejecutar ningún otro comando.

**Acceptance Scenarios**:

1. **Given** un proyecto con el proveedor de grafo externo instalado, **When** el usuario ejecuta el comando de indexado sin ninguna opción adicional, **Then** el grafo propio se actualiza como hoy y, a continuación, el grafo externo también se actualiza, y el comando informa cuántos nodos y relaciones quedaron en el grafo externo.
2. **Given** un proyecto sin el proveedor de grafo externo instalado, **When** el usuario ejecuta el comando de indexado, **Then** el grafo propio se actualiza con normalidad, se informa que el grafo externo se omitió por no estar instalado, y el comando termina en éxito.
3. **Given** un proveedor de grafo externo instalado pero que falla al reindexar (por ejemplo, un error inesperado de la herramienta externa), **When** el usuario ejecuta el comando de indexado, **Then** el grafo propio se actualiza con normalidad, se muestra una advertencia sobre el fallo del grafo externo, y el comando igual termina en éxito.
4. **Given** un usuario que solo quiere actualizar el grafo propio (por ejemplo, para ir más rápido), **When** ejecuta el comando de indexado con la opción de omitir el grafo externo, **Then** solo se actualiza el grafo propio y no aparece ninguna línea relacionada con el grafo externo.

---

### User Story 2 - Refrescar el grafo externo desde la interfaz interactiva (Priority: P2)

Como usuario que trabaja dentro de la interfaz interactiva (TUI) de `mem`, quiero poder disparar el mismo refresco del grafo de código externo desde ahí, sin tener que salir a una terminal aparte y ejecutar comandos por su cuenta.

**Why this priority**: Extiende el valor de la Historia 1 a quienes viven principalmente en la interfaz interactiva, pero depende de que esa capacidad ya exista como acción invocable (Historia 1), por lo que va después.

**Independent Test**: Puede probarse completamente abriendo la interfaz interactiva, disparando la acción de reindexar el grafo externo desde la pantalla de configuración, y verificando que el resultado (éxito con conteos, "no disponible", o fallo) se refleja en la interfaz sin haber usado la línea de comandos.

**Acceptance Scenarios**:

1. **Given** el proveedor de grafo externo instalado, **When** el usuario selecciona la acción "reindexar grafo externo" en la pantalla de configuración de la interfaz interactiva, **Then** la interfaz muestra de inmediato que el reindexado comenzó, sigue respondiendo a otras teclas mientras el proceso corre en segundo plano, y al terminar muestra el resultado (nodos y relaciones actualizados).
2. **Given** el proveedor de grafo externo NO instalado, **When** el usuario selecciona esa misma acción, **Then** la interfaz informa que el grafo externo no está disponible, sin intentar el reindexado ni quedarse bloqueada.
3. **Given** un reindexado del grafo externo ya en curso, **When** el usuario intenta disparar la misma acción otra vez, **Then** la interfaz no inicia un segundo reindexado simultáneo y deja claro que ya hay uno en curso.

---

### User Story 3 - Editar la huella de contexto sin salir de la interfaz interactiva (Priority: P3)

Como usuario que quiere ajustar cuánto contexto retiene y compacta `mem` (presupuesto, umbral de compactación, ventana de deduplicación), quiero poder cambiar esos tres valores directamente desde la pantalla de configuración de la interfaz interactiva, en vez de tener que editar a mano un archivo de configuración.

**Why this priority**: Es una mejora de usabilidad independiente de las dos anteriores — hoy esos valores ya son visibles en la interfaz, solo faltan ser editables. Aporta valor por sí sola pero es la de menor urgencia de las tres.

**Independent Test**: Puede probarse completamente entrando a la pantalla de configuración, editando cada uno de los tres valores (incluyendo casos límite como cero y negativo), y verificando que el nuevo valor se guarda, se refleja de inmediato en el resumen de la pantalla, y persiste al volver a abrir la aplicación.

**Acceptance Scenarios**:

1. **Given** la pantalla de configuración de la interfaz interactiva, **When** el usuario selecciona uno de los tres ajustes de huella de contexto, **Then** se abre una pantalla de edición precargada con el valor actual de ese ajuste.
2. **Given** la pantalla de edición de un ajuste, **When** el usuario escribe un número entero (positivo, cero o negativo) y confirma, **Then** el valor se guarda, la aplicación vuelve a la pantalla de configuración y el resumen muestra el nuevo valor de inmediato.
3. **Given** la pantalla de edición de un ajuste, **When** el usuario escribe un valor vacío o no numérico y confirma, **Then** se muestra un mensaje de error claro, el valor no se guarda, y el usuario permanece en la pantalla de edición para corregirlo.
4. **Given** la pantalla de edición de un ajuste con cambios sin confirmar, **When** el usuario cancela la edición, **Then** la aplicación vuelve a la pantalla de configuración y el valor guardado no cambia.

---

### Edge Cases

- ¿Qué pasa si el usuario ejecuta el comando de indexado en un proyecto que el proveedor externo nunca había visto antes (primera vez)? El reindexado externo debe funcionar igual, registrando el proyecto como parte del propio proceso de indexado, sin requerir un paso de registro previo.
- ¿Qué pasa si el reindexado del grafo externo tarda varios minutos? El comando de línea de comandos debe esperar a que termine antes de reportar el resultado; la interfaz interactiva, en cambio, no debe quedarse congelada mientras tanto.
- ¿Qué pasa si el usuario dispara el reindexado externo desde la interfaz y cierra o navega a otra pantalla antes de que termine? El proceso debe seguir corriendo en segundo plano y el resultado debe reflejarse cuando el usuario vuelva a la pantalla de configuración (o mediante un aviso de estado visible desde cualquier pantalla).
- ¿Qué pasa si el usuario introduce un valor decimal (no entero) en la edición de un ajuste de huella de contexto? Debe tratarse igual que un valor no numérico: error claro, sin guardar.
- ¿Qué pasa si dos de los tres ajustes de huella de contexto se editan en la misma sesión, uno tras otro? Cada edición debe guardarse de forma independiente sin afectar a los otros dos valores.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir refrescar, con una sola invocación del comando de indexado, tanto el grafo de código propio como el grafo de código externo (cuando el proveedor opcional está instalado).
- **FR-002**: El sistema DEBE permitir al usuario optar por omitir el refresco del grafo externo en esa misma invocación, actualizando solo el grafo propio.
- **FR-003**: Cuando el proveedor de grafo externo no esté instalado, el sistema DEBE informarlo con un mensaje claro y continuar sin marcar el comando como fallido.
- **FR-004**: Cuando el refresco del grafo externo falle por cualquier otro motivo, el sistema DEBE reportarlo como una advertencia, sin que el comando en su conjunto se marque como fallido (el refresco del grafo propio ya se completó con éxito).
- **FR-005**: Al finalizar con éxito el refresco del grafo externo, el sistema DEBE informar cuántos nodos y relaciones quedaron en ese grafo.
- **FR-006**: El refresco del grafo externo disparado por el comando de indexado DEBE registrar el proyecto en el proveedor externo aunque sea la primera vez que se indexa (no debe depender de un registro previo).
- **FR-007**: La interfaz interactiva DEBE ofrecer una acción, accesible desde la pantalla de configuración, equivalente al refresco del grafo externo del comando de indexado.
- **FR-008**: Mientras el refresco del grafo externo dispara desde la interfaz interactiva está en curso, la interfaz DEBE seguir respondiendo a la interacción del usuario (no debe bloquearse ni congelarse).
- **FR-009**: La interfaz interactiva DEBE mostrar cuándo el refresco del grafo externo comienza y cuál fue su resultado final (éxito con conteos, no disponible, o fallo) al terminar.
- **FR-010**: Cuando el proveedor de grafo externo no esté disponible, la acción equivalente en la interfaz interactiva DEBE informarlo sin intentar el refresco.
- **FR-011**: La interfaz interactiva NO DEBE permitir iniciar un segundo refresco del grafo externo mientras uno ya está en curso; debe dejar claro al usuario que ya hay uno en marcha.
- **FR-012**: La interfaz interactiva DEBE permitir ver y editar, sin salir de la aplicación, los tres ajustes de huella de contexto (presupuesto, umbral de compactación, ventana de deduplicación).
- **FR-013**: Al editar uno de esos ajustes, la interfaz DEBE precargar el valor actual como punto de partida.
- **FR-014**: El sistema DEBE validar que el valor introducido al editar un ajuste sea un número entero; un valor vacío, no numérico o decimal DEBE rechazarse con un mensaje de error claro, sin guardar el cambio.
- **FR-015**: El sistema DEBE aceptar valores enteros en cero o negativos para estos tres ajustes, conservando su significado ya existente (cero usa el valor por defecto, negativo desactiva explícitamente el límite correspondiente).
- **FR-016**: El usuario DEBE poder cancelar la edición de un ajuste sin que se guarde ningún cambio.
- **FR-017**: Tras guardar un ajuste con éxito, la interfaz DEBE reflejar de inmediato el nuevo valor en el resumen de la pantalla de configuración.

### Key Entities

- **Grafo de código propio**: representación estructural del código generada internamente por `mem`, limitada a archivos del lenguaje propio del proyecto.
- **Grafo de código externo**: representación estructural del código generada por un proveedor opcional de terceros, con cobertura multi-lenguaje; su disponibilidad depende de que ese proveedor esté instalado.
- **Ajustes de huella de contexto**: los tres valores numéricos que controlan cuánta información retiene y compacta `mem` al construir contexto (presupuesto, umbral de compactación, ventana de deduplicación); cada uno admite un valor por defecto (cero) o una desactivación explícita (negativo).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un usuario puede refrescar ambos grafos de código con una sola acción, en lugar de los dos pasos manuales separados que se requieren hoy.
- **SC-002**: El comando principal de indexado termina en éxito el 100% de las veces en que el proveedor de grafo externo no está instalado, sin ninguna regresión respecto al comportamiento actual del grafo propio.
- **SC-003**: Un usuario puede iniciar y monitorear el refresco del grafo externo completamente desde la interfaz interactiva, sin salir a una terminal aparte, en menos de 10 segundos desde que decide hacerlo hasta que ve confirmación de que empezó.
- **SC-004**: Un usuario puede cambiar cualquiera de los tres ajustes de huella de contexto desde la interfaz interactiva, sin editar archivos a mano, en menos de 30 segundos por ajuste.
- **SC-005**: El 100% de los intentos de guardar un valor inválido (vacío, no numérico o decimal) en un ajuste de huella de contexto se rechaza antes de persistirse, sin corromper la configuración existente.

## Assumptions

- El proveedor de grafo externo, cuando está instalado, es el mismo ya integrado en el proyecto; esta funcionalidad no agrega soporte para proveedores nuevos.
- El refresco del grafo externo siempre corre en modo completo; esta funcionalidad no expone al usuario ninguna opción de modo parcial o incremental.
- El resultado del comando de indexado (éxito/fallo) refleja únicamente el estado del grafo propio; el resultado del grafo externo es siempre informativo y nunca hace fallar el comando completo.
- Los ajustes de huella de contexto aplican por proyecto, igual que hoy; esta funcionalidad no cambia ese alcance.
- La semántica ya existente de los tres ajustes (cero = valor por defecto, negativo = desactivación explícita) no cambia; esta funcionalidad solo cambia cómo se editan, no lo que significan.
