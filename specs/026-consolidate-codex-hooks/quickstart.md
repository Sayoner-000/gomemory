# Guía de validación: Consolidación de hooks de Codex

## Prerrequisitos

- Codex CLI 0.149.1 o la versión instalada que incluya la función estable `hooks`.
- Acceso de lectura y escritura a `/root/.codex`.
- Destinos y entornos requeridos por cada hook del inventario actual.
- Un mecanismo de observación por identidad; para hooks de socket puede ser el receptor real o un colector temporal.

## 1. Capturar el estado previo

Crear un directorio temporal con marca de tiempo y copiar allí `config.toml` y `hooks.json`. Registrar sus hashes y
conservar el directorio hasta finalizar todas las verificaciones.

Resultados esperados:

- El respaldo contiene ambos archivos originales.
- Se pueden comparar hashes antes y después del cambio.

## 2. Instalar y validar el candidato antes de retirar el JSON

Comprobar el candidato contra [contracts/hooks-config.md](contracts/hooks-config.md), escribirlo en un archivo temporal
del mismo directorio y reemplazar `config.toml` de forma atómica. Mantener `hooks.json` y el respaldo intactos mientras
se ejecuta la carga estricta sobre la configuración activa:

```bash
codex --strict-config doctor --json
```

Resultados esperados:

- `.checks["config.load"].status` es `ok`.
- `.checks["config.load"].details["config.toml parse"]` es `ok`.
- Los fallos de red, si el entorno restringe acceso externo, se registran por separado y no invalidan esta prueba de
  configuración.
- Si la carga falla, se restaura inmediatamente `config.toml` desde el respaldo antes de continuar.

## 3. Activar la fuente única

Mover `hooks.json` al directorio de respaldo, fuera de `/root/.codex/hooks.json`, y abrir una sesión nueva de Codex.
Si Codex solicita confianza para hooks migrados o reubicados, revisar cada acción y autorizarla desde su nueva
procedencia. No calcular ni trasladar manualmente hashes de confianza.

Resultados esperados:

- No aparece `loading hooks from both /root/.codex/hooks.json and /root/.codex/config.toml`.
- El aviso de descubrimiento de código aparece una sola vez.
- La evidencia definida para cada identidad muestra una sola ejecución aplicable a la sesión nueva.

## 4. Verificar todos los eventos y el estado final

Probar inicio, reanudación, limpieza y compactación. Para cada evento, registrar su `source`, el identificador de sesión,
el conteo observable de cada identidad. La matriz debe mostrar exactamente una ejecución aplicable por hook y evento.
Inspeccionar después `config.toml` con un lector TOML y confirmar:

- Una sola definición por identidad normalizada del inventario original.
- Conservación de eventos, filtros y todos los campos compatibles de cada acción.
- Ninguna clave de confianza asociada a `hooks.json`.
- Una entrada vigente de confianza para cada hook autorizado desde `config.toml`, con hash no vacío y habilitada según
  la serialización de Codex.
- Ningún cambio en modelo, servidores MCP, confianza del proyecto ni otras funciones.

Repetir la normalización sobre el resultado y confirmar que no cambia el inventario ni el archivo consolidado.

## 5. Rollback si falla una verificación

Cerrar todas las sesiones de prueba que puedan persistir confianza. Restaurar de forma atómica desde el directorio
temporal tanto `config.toml` como `hooks.json`, comprobar que ambos hashes coinciden con la línea base y volver a
ejecutar `codex --strict-config doctor --json`. No mezclar el archivo consolidado con el estado de confianza antiguo.

El respaldo solo puede eliminarse después de que todas las comprobaciones anteriores hayan pasado y ya no se requiera
recuperación inmediata.
