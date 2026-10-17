# Generación por harness: verificación del 17 de octubre de 2026

Esta evidencia actualiza la investigación inicial de generación local. No
habilita intercepción ni otras capacidades pendientes de la matriz.

## Claude Code 2.1.302

El esquema canónico provoca `no schema with key or ref
"https://json-schema.org/draft/2020-12/schema"`. El adaptador omite únicamente
`$schema` de su copia para transporte; conserva las restricciones del contrato.

```sh
GOMEMORY_TEST_REAL_HARNESS=claude go test ./adapters/secondary/generator -run '^TestRealSocraticaHarness$' -v -count=1
```

Resultado: PASS, 3 preguntas y propuesta recibida, 20.24 s. La prueba usa el
prompt de proyectos de grafos visuales de código abierto y no despacha la tarea.

## Codex CLI 0.163.0

La copia original del esquema produce HTTP 400: falta `scrutiny` en `required`.
El adaptador transforma las propiedades opcionales en requeridas anulables,
recursivamente, sin cambiar el esquema canónico del dominio.

```sh
GOMEMORY_TEST_REAL_HARNESS=codex go test ./adapters/secondary/generator -run '^TestRealSocraticaHarness$' -v -count=1
```

Resultado: PASS, 2 preguntas y propuesta recibida, 85.40 s. La prueba conserva
el sandbox de solo lectura y no despacha la tarea.

## OpenCode V2

El adaptador crea un agente primario `socratica-generator` en un directorio
temporal, con `permissions: [{action: "*", resource: "*", effect: deny}]`.
Usa `run --standalone --agent socratica-generator --format json` con la
petición por stdin, como claude y codex: el prompt no se escribe en disco
(cierre H-02 de la auditoría 2026-10-17). Las pruebas locales verifican el
perfil, la petición por stdin, la ausencia de `--file` y de otros archivos en
el directorio, su limpieza y el rechazo de eventos de herramientas o de error.
La verificación de versión solo se cachea si tiene éxito (cierre H-01).

```sh
GOMEMORY_TEST_REAL_HARNESS=opencode go test ./adapters/secondary/generator -run '^TestRealSocraticaHarness$' -v -count=1
```

Resultado real: FAIL, `provider.auth`, HTTP 403, `FreeTierError` («OpenCode's free
tier can only be used from within OpenCode»). No se cambió de proveedor, modelo
ni harness. La generación y la restricción efectiva de herramientas en una
sesión de modelo quedan pendientes de una cuenta/proveedor que acepte la
petición; el error estructurado se muestra a la persona conservando el original.

Referencia del perfil y los permisos: [agentes OpenCode V2](https://opencode.ai/v2/docs/agents/).
