# Investigación: Consolidación de hooks de Codex

## Decisión 1: Usar `config.toml` como fuente única

**Decisión**: Mantener todas las definiciones en `/root/.codex/config.toml` y retirar `/root/.codex/hooks.json` de la
ruta que Codex reconoce.

**Justificación**: La advertencia observada identifica explícitamente la carga simultánea de ambas representaciones.
La instalación actual ya carga correctamente hooks desde TOML y tiene habilitada la función estable `hooks`.

**Alternativas consideradas**:

- Conservar solo `hooks.json`: descartado porque obligaría a mover el hook ya operativo de memoria de código al
  formato heredado y mantendría separada la configuración principal.
- Mantener ambos archivos sin duplicados: descartado porque no elimina la advertencia ni la ambigüedad de fuentes.

## Decisión 2: Migrar por estructura, no por proveedor

**Decisión**: Inventariar y trasladar todos los eventos, grupos, acciones y campos compatibles sin inspeccionar nombres
de proveedor ni contenido de comandos.

**Justificación**: La equivalencia funcional depende de evento, filtro y campos de acción. El estado actual incluye un
hook sin `matcher` y con `timeout = 10`; esos valores se conservan porque forman parte de sus datos, no porque el hook
pertenezca a Herdr.

**Alternativas consideradas**:

- Mantener una lista de proveedores conocidos: descartado porque perdería hooks nuevos o locales.
- Aplicar valores predeterminados explícitos al migrar: descartado porque puede cambiar la semántica de campos ausentes.

## Decisión 3: Renovar confianza según identidad y posición

**Decisión**: Conservar un estado confiable solo cuando fuente, evento, posición e identidad no cambien. Eliminar estados
de duplicados y de la fuente retirada; Codex vuelve a autorizar cualquier hook migrado o reubicado.

**Justificación**: Una posición puede representar otra identidad después de deduplicar. Mantener su hash asociaría el
nuevo hook con una autorización anterior; trasladar hashes entre fuentes asumiría que la confianza es portable.

**Alternativas consideradas**:

- Copiar hashes desde cualquier fuente: descartado por no existir evidencia de que sean portables.
- Eliminar todo `[hooks.state]`: descartado porque forzaría una reautorización innecesaria del hook que no cambia.

## Decisión 4: Aplicar una transición recuperable

**Decisión**: Respaldar ambos archivos en un directorio temporal con marca de tiempo, validar primero el TOML y solo
después mover `hooks.json` fuera de su nombre reconocido.

**Justificación**: Permite volver exactamente al estado anterior si falla la carga, la autorización o el comportamiento
de cualquier destino, y cumple el requisito de confirmar la migración antes de retirar la representación antigua.

**Alternativas consideradas**:

- Borrar el JSON inmediatamente: descartado porque reduce innecesariamente la recuperabilidad.
- Dejar `hooks.json.bak` junto a la configuración sin respaldo externo: viable, pero un directorio temporal dedicado
  conserva juntas ambas versiones originales y hace explícito el rollback.

## Decisión 5: Validar con herramientas locales y sesiones reales

**Decisión**: Usar carga estricta, `codex doctor --json`, inspección de definiciones y sesiones reales para verificar la
consolidación.

**Justificación**: La documentación oficial de OpenAI consultada no expuso una página pública específica para esta
advertencia. La CLI instalada sí confirma que `hooks` es estable, que el TOML actual carga y que el contrato candidato
es aceptado. Solo una sesión real puede demostrar la ausencia del aviso y la ejecución efectiva de los hooks.

**Alternativas consideradas**:

- Validación exclusivamente visual del archivo: descartada porque un TOML legible puede no coincidir con el esquema.
- Confiar solo en `codex doctor`: descartado porque el diagnóstico no demuestra por sí solo la emisión única ni el
  efecto observable de cada hook externo.

## Decisión 6: Definir observabilidad por hook

**Decisión**: Asociar a cada identidad una evidencia observable adecuada a su acción. Para el inventario actual, el
aviso se cuenta en la salida y el hook de socket se verifica en su receptor o con un colector temporal.

**Justificación**: Un código de salida correcto no demuestra necesariamente el efecto de un hook. La estrategia debe
derivarse de la acción inventariada, no de una única integración.

**Alternativas consideradas**:

- Ejecutar todos los comandos sin entrada de evento: descartado porque puede omitir datos requeridos.
- Usar únicamente la ausencia de errores: descartado porque algunos destinos se degradan sin error visible.
