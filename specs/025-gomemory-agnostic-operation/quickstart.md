# Guía rápida de validación

## Prerrequisitos

- Go 1.25.0 o el toolchain definido por el proyecto.
- Un clon del repositorio y dependencias disponibles.
- Para validar la configuración MCP sin tocar la cuenta personal, un directorio temporal asignado como
  directorio personal del proceso de prueba.

## 1. Validación automatizada

Desde la raíz del repositorio:

```bash
go test ./adapters/primary/cli ./adapters/primary/tui -count=1
go test ./... -count=1
go vet ./...
go build -o /tmp/gomemory-mem ./infrastructure/
```

Resultado esperado: todas las pruebas y verificaciones terminan correctamente. Las pruebas de CLI
comprueban migración, conservación, respaldo, permisos e idempotencia; las de TUI comprueban pegado,
desplazamiento y copia íntegra.

## 2. Registro MCP de Codex

1. En una configuración de prueba, declare registros heredados de gomemory y al menos un servidor
   ajeno.
2. Ejecute la instalación o configuración MCP de gomemory dos veces.
3. Compruebe que existe un único registro compartido, que no depende de una ruta de proyecto y que el
   servidor ajeno no cambió.
4. Compruebe que existe un respaldo cuando había registros heredados y que los permisos se preservan.
5. Inicie Codex desde un directorio diferente, incluido uno creado después de la migración.

Resultado esperado: gomemory inicia sin error de directorio inexistente y no aparecen registros por
proyecto.

## 3. Interacción textual de la TUI

1. Ejecute la TUI y abra una memoria con más líneas que el alto de la terminal.
2. Recorra el detalle por línea, página, inicio y final.
3. Solicite copiar desde el detalle estando tanto al inicio como al final.
4. Pegue texto con caracteres multibyte en cada formulario y campo de edición disponible.

Resultado esperado: el detalle alcanza ambos extremos, los indicadores muestran contenido fuera de
vista, la copia conserva todo el texto y cada campo activo recibe el contenido pegado.

## 4. Constitución predeterminada

1. Consulte o restaure la constitución predeterminada de un proyecto de prueba.
2. Compruebe que no incluye título, autoría ni pertenencia particular.
3. Importe una versión personalizada, ejecute de nuevo la inicialización y vuelva a consultarla.
4. Si el proyecto usa spec-kit, sincronice la constitución y compruebe que se refleja sin crear una
   estructura nueva en un proyecto que no la tenga.

Resultado esperado: el valor predeterminado es agnóstico y el contenido personalizado permanece
intacto hasta que la persona solicite una restauración explícita.
