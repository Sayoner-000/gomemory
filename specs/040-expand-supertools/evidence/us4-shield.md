# US4 — escaneo real (T060)

2026-10-21, macOS, binario recompilado desde el árbol de trabajo, versión 2.35.0 + 040.

Orden: `mem doctor --shield --json`. Resultado: exit 2, cero archivos skipped, un hallazgo secret/critical y uno mcp-inline-credential/high en la configuración de OpenCode.

Se verificó que corresponden a una credencial literal en la clave `mcp.context7.headers.CONTEXT7_API_KEY`. No se mostró ni copió su valor. Es un hallazgo verdadero; no fue necesario excluirlo ni modificar el detector. La configuración se mantuvo intacta.

Propuesta de cierre operativo: sustituir el literal por la referencia de entorno admitida por el harness y repetir doctor. Esta acción de configuración queda a cargo de la persona; shield es de solo lectura.

Pruebas: fixtures versionados en `tests/testdata/shield`, secretos sintéticos, referencias de entorno, permisos de gomemory y binarios del PATH sin falsos positivos; lector sin cambios de contenido o mtime; escaneo automático una vez por huella.
