# Contrato de instalación y actualización

Requisitos FR-011–015; Bash/macOS/Linux y PowerShell/Windows. No reemplaza contratos de presentación o event stream actuales.

## Resolución e integridad

Una selección latest se resuelve una vez a tag estable concreto. Mantener selección explícita actual y rechazar tags preliminares como nuevo contrato de soporte. El artefacto y checksums.txt se descargan desde el mismo release resuelto.

La entrada del asset en checksums.txt debe ser única y contener exactamente un digest SHA-256 válido. Aceptar separador de texto estándar y asterisco opcional de modo binario; comparar nombre completo exacto, nunca un substring. Entrada duplicada se rechaza aunque ambos digests coincidan. No extraer antes de comparar digest.

Bash usa sha256sum o shasum -a 256; ausencia de ambos falla. PowerShell usa Get-FileHash -Algorithm SHA256. Comparación del digest no distingue mayúsculas. Checksum del mismo origen no es firma independiente ni prueba de autenticidad fuera de ese origen.

Antes de publicar: ejecutar extracción solo para miembro permitido mem/mem.exe, validar archivo regular, no vacío, no enlace/reparse inseguro y utilizable en la plataforma. Rechazar traversal, rutas absolutas y miembros no esperados que afectarían el destino. El script no ejecuta el binario descargado antes de verificarlo.

Conservación: fallos de resolución, red, comprobante, hash, extracción, validación o permisos previos a publicación dejan instalación anterior intacta. No cambiar PATH ni configuración como si se hubiese instalado. Mantener cleanup del temporal y presentar fallo depurado.

## Publicación de update Unix

Preparar temporal en el mismo directorio del ejecutable resuelto por el contrato vigente. Copiar fuente verificada, establecer permisos ejecutables, sincronizar y cerrar. Solo entonces rename al destino público. Cancelación/error anterior a rename elimina el temporal propio y conserva el instalado. Error de rename preserva destino anterior y no declara éxito.

Una invocación concurrente encuentra ejecutable anterior completo o nuevo completo. Pruebas de preparación controlada no dependen de dormir un tiempo arbitrario: usan barreras para observar la etapa de copia. No se exige reiniciar procesos antiguos ni garantía nueva frente a pérdida de energía.

La actualización Windows mantiene su política existente para binario en uso. El instalador PowerShell sí debe demostrar integridad y conservación antes de publicación en Windows; no se promete atomicidad Unix mediante un rename Windows.

## Matriz de aceptación

Cada script: explícita y latest; hash correcto; incorrecto; ausente; inválido; entrada ausente; duplicado; red fallida; herramienta verificadora ausente donde aplique; archivo vacío; permisos insuficientes; ejecutable previo. Update Unix: fuente ausente; copia bloqueada/parcial; cancelación; chmod/sync/close/rename fallidos e invocaciones durante preparación.

Descargas de prueba: fixtures locales interceptan funciones de red sin añadir un switch de producción para desactivar verificación. Bash usa PATH/curl de fixture; PowerShell usa funciones controladas en un proceso aislado y LOCALAPPDATA temporal. No debilitar el contrato real de scripts para facilitar mocks.
