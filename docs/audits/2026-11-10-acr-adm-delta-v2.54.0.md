# Informe ACR + ADM: delta posterior a v2.54.0

**Fecha:** 2026-11-10
**Revisión auditada:** `973bd94` (árbol limpio)
**Alcance:** delta `c4995cc..973bd94` no cubierto por `2026-11-10-acr-adm-v2.54.0.md`: `clikit/mcp.go` (`typeForStrictClients`), `domain/rules_graph.go` (`absolutePath`), `cli/i18n.go` (catálogo vacío sin caché), `domain/seed.go` (workrules-v3), plugin OpenCode (bloque NATIVE TOOLS).
**Independencia ACR:** degradada. Octopus enrutó INLINE; las dos pasadas (A: esquemas MCP e i18n; B: reglas de rutas y semilla) las hizo el mismo agente y modelo.

## 1. Resumen

- **Tecnologías:** Go (producción y pruebas), TypeScript (plugin OpenCode).
- **Riesgo general:** Bajo.
- **Resultado:** 0 críticos, 0 altos, 0 medios y 2 bajos (SOSPECHOSO). No hubo hallazgos CONFIRMED, así que no se aplicó ninguna corrección.

| Comprobación | Resultado |
|---|---|
| `go vet ./...` | Pasa |
| `gofmt -l` (adapters, domain, application, infrastructure) | Sin salida |
| `go test -race -count=1` domain, usecases, cli, clikit | Pasa |
| `go test` integración Seed/Bootstrap | Pasa |
| `govulncheck`, `staticcheck`, `gitleaks` | No disponibles en el host (no instalados) |

## 2. Hallazgos

| ID | Severidad | Estado | Problema |
|---|---|---|---|
| S-01 | Baja | SOSPECHOSO | `typeForStrictClients` publica como `object` cualquier subesquema sin tipo |
| S-02 | Baja | SOSPECHOSO | Rutas de Windows relativas a una unidad (`C:foo`) y separadores `\` en `joinShellPath` |

### S-01: subesquemas sin tipo publicados como `object`
- **Ubicación:** `adapters/primary/cli/clikit/mcp.go` (`typeForStrictClients`).
- **Análisis:** cualquier campo `any` futuro que acepte un escalar o un array quedará restringido a `object`. Si el SDK valida la entrada contra ese esquema, lo rechazará. Hoy el único `any` de entrada es `socraticaPreparationInput.Value`, y el dominio exige que sea un objeto, así que no tiene impacto actual.
- **Cierre propuesto:** añadir a `cmd_mcp_schemas_test.go` una prueba que falle si aparece un campo `any` de entrada nuevo y no está en una lista blanca.

### S-02: `C:foo` y `src\x.go` en las reglas de rutas
- **Ubicación:** `domain/rules_graph.go` (`absolutePath`, `joinShellPath`).
- **Análisis:** `C:foo` se trata como relativa y se une a la ruta con `path.Join`, que solo entiende `/`. El resultado es una ruta mal formada. El efecto se limita a la elección de cobertura en el hook de consejo: no es una frontera de seguridad.
- **Cierre propuesto:** normalizar `\` a `/` antes de evaluar la ruta y añadir casos de tabla en `rules_graph_test.go`.

### Observación (INFO)
- En `cli/i18n.go`, si el catálogo embebido falla, el respaldo carga locales desde rutas relativas al directorio de trabajo, que podría ser el repositorio del usuario. En producción no se llega a ese punto porque `TemplatesFS` siempre está definido. Una mejora opcional sería limitar ese respaldo a las pruebas.

## 3. Veredicto ACR

```
Confirmed: 0 · Suspect: 2 (LOW) · Contradictions: 0 · Info: 1
Corrections: none · Re-judgment: n/a
VERDICT: APPROVED (independence: degraded)
```

El veredicto se refiere solo a esta revisión. No autoriza commit, push ni release.

## 4. Recomendaciones
- Instalar `govulncheck` y `gitleaks` en CI, o confirmar que ya se ejecutan allí, para cubrir la cadena de suministro y los secretos.
- Fijar con pruebas S-01 y S-02 antes de ampliar las reglas a Windows.
