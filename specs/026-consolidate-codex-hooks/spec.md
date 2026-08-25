# Especificación de funcionalidad: Consolidación de hooks de Codex

**Rama de funcionalidad**: `main`

**Creado**: 2026-08-24

**Estado**: Implementado

**Entrada**: Consolidar de forma agnóstica la configuración de hooks de Codex en una sola fuente, preservar todos los hooks existentes y eliminar definiciones equivalentes duplicadas.

## Escenarios de usuario y pruebas *(obligatorio)*

### Historia de usuario 1 - Iniciar Codex sin configuración ambigua (Prioridad: P1)

Como persona que administra Codex, quiero que los hooks se carguen desde una sola fuente para iniciar una sesión sin advertencias de representaciones múltiples ni comportamiento ambiguo.

**Por qué esta prioridad**: Eliminar la ambigüedad es el objetivo principal y evita que distintas definiciones compitan durante el inicio.

**Prueba independiente**: Se puede abrir una sesión nueva y comprobar que la carga de hooks usa una sola representación y no muestra la advertencia sobre `hooks.json` y `config.toml`.

**Escenarios de aceptación**:

1. **Dado** que la configuración consolidada está activa, **cuando** se inicia o reanuda Codex, **entonces** no aparece la advertencia `loading hooks from both /root/.codex/hooks.json and /root/.codex/config.toml`.
2. **Dado** que se inspeccionan las fuentes de configuración activas, **cuando** se enumeran las definiciones de hooks, **entonces** todas proceden de `/root/.codex/config.toml`.

---

### Historia de usuario 2 - Preservar todos los hooks existentes (Prioridad: P2)

Como persona que administra automatizaciones de Codex, quiero que todos los hooks existentes conserven su comportamiento después de la consolidación, sin depender del proveedor o del comando que ejecuten.

**Por qué esta prioridad**: Resolver la advertencia no debe eliminar ni alterar automatizaciones válidas que ya forman parte del ciclo de sesión.

**Prueba independiente**: Se puede comparar el inventario normalizado antes y después, iniciar una sesión y verificar que cada hook único se ejecuta una vez con los mismos filtros, comandos, límites y opciones.

**Escenarios de aceptación**:

1. **Dado** un hook válido definido en cualquiera de las fuentes originales, **cuando** finaliza la consolidación, **entonces** existe una definición funcionalmente equivalente en la fuente única.
2. **Dado** un hook con filtros, límites u opciones adicionales, **cuando** se migra, **entonces** esos atributos se conservan sin reglas específicas para su proveedor o comando.

---

### Historia de usuario 3 - Evitar ejecuciones equivalentes duplicadas (Prioridad: P3)

Como persona que usa hooks de Codex, quiero que cada comportamiento equivalente se ejecute una sola vez por evento para evitar ruido y efectos secundarios duplicados.

**Por qué esta prioridad**: La duplicación actual no impide iniciar Codex, pero dificulta distinguir el comportamiento esperado de una configuración defectuosa.

**Prueba independiente**: Se puede iniciar, reanudar, limpiar o compactar una sesión y confirmar una sola ejecución de cada identidad normalizada por evento.

**Escenarios de aceptación**:

1. **Dado** cualquiera de los eventos admitidos, **cuando** se procesan los hooks, **entonces** cada identidad normalizada se ejecuta exactamente una vez.
2. **Dado** que se inspecciona la configuración consolidada, **cuando** se agrupan definiciones funcionalmente equivalentes, **entonces** cada grupo contiene una sola definición activa.

### Casos límite

- Si el destino de un hook no existe o no puede ejecutarse, la consolidación sigue siendo válida y el fallo debe ser identificable sin restaurar una segunda fuente.
- Si aparecen tipos de evento, acciones u opciones no conocidos de antemano, deben conservarse como datos y no descartarse por falta de reglas específicas.
- Si permanece estado de confianza asociado al archivo retirado, este no debe reactivar ni representar una definición de hook obsoleta.
- Si la configuración principal no es válida después del cambio, debe detectarse antes de retirar definitivamente la configuración anterior.
- Reiniciar, reanudar, limpiar o compactar una sesión no debe multiplicar hooks equivalentes ni sus mensajes.

## Requisitos *(obligatorio)*

### Requisitos funcionales

- **RF-001**: El sistema DEBE mantener una única fuente activa para todas las definiciones de hooks de Codex.
- **RF-002**: La fuente única DEBE conservar habilitada la funcionalidad de hooks existente.
- **RF-003**: Cada hook único presente en cualquier fuente original DEBE conservar evento, filtro, tipo de acción, comando, límite y opciones compatibles.
- **RF-004**: Cada identidad funcional de hook DEBE tener exactamente una definición activa después de normalizar equivalencias.
- **RF-005**: El sistema NO DEBE conservar referencias de estado activas hacia una representación de hooks retirada.
- **RF-006**: La representación anterior NO DEBE retirarse hasta confirmar que su comportamiento necesario está presente en la fuente consolidada.
- **RF-007**: Una sesión nueva DEBE iniciar sin advertencias por carga simultánea de múltiples representaciones de hooks.
- **RF-008**: La consolidación NO DEBE modificar configuraciones ajenas a los hooks involucrados.
- **RF-009**: La migración DEBE operar sobre la estructura y los campos de los hooks, sin decisiones basadas en nombres de proveedores, comandos o scripts concretos.
- **RF-010**: `mem install` y la configuración global explícita de Codex DEBEN ejecutar la consolidación para que la capacidad esté disponible en instalaciones públicas de GoMemory.
- **RF-011**: Un `hooks.json` ilegible o inválido DEBE conservarse intacto y NO DEBE impedir que GoMemory registre su servidor MCP en Codex.
- **RF-012**: La migración DEBE ser idempotente: repetir la instalación no debe recrear `hooks.json` ni duplicar grupos consolidados.

### Entidades clave

- **Fuente de configuración de hooks**: Representación activa que reúne eventos, filtros, acciones, límites de tiempo y estado de habilitación.
- **Hook de sesión**: Acción vinculada a uno o más eventos del ciclo de sesión; se identifica por su propósito, comando y condiciones de ejecución.
- **Estado de confianza**: Registro que autoriza o habilita una definición concreta y debe corresponder únicamente a fuentes vigentes.

## Criterios de éxito *(obligatorio)*

### Resultados medibles

- **CE-001**: En el 100 % de las sesiones de prueba posteriores al cambio se observan cero advertencias por carga desde dos representaciones de hooks.
- **CE-002**: El 100 % de los hooks únicos del inventario original aparece con semántica equivalente en la fuente consolidada.
- **CE-003**: Cada identidad normalizada se ejecuta exactamente una vez por cada evento aplicable probado.
- **CE-004**: El 100 % de las definiciones activas y sus referencias de estado corresponden a la fuente consolidada.
- **CE-005**: Una persona administradora puede confirmar la consolidación y los dos comportamientos preservados en menos de cinco minutos siguiendo la verificación documentada.

## Suposiciones

- Ninguna herramienta externa depende de la existencia física de `/root/.codex/hooks.json`.
- La configuración principal de Codex admite los tipos de hook y campos presentes en las fuentes actuales.
- En el estado observado, Herdr y `codebase-memory-mcp` son casos de prueba concretos, no dependencias del mecanismo de consolidación.
- La corrección se limita a consolidar los hooks descritos; el modelo, los servidores MCP, la confianza del proyecto y otras preferencias quedan fuera del alcance.
- La retirada de la representación anterior se hará de forma recuperable o después de conservar una copia temporal hasta completar la verificación.
