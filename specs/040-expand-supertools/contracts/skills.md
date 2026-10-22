# Contrato: flow-skills de expand

## Ubicación

```text
infrastructure/templates/expand/
├── NOTICE                      # atribución MIT de ECC (affaan-m/ECC v2.2.3)
├── evolve-prompt.md            # plantilla de evolve (R13); no es una skill
├── verify-loop/SKILL.md
├── search-first/SKILL.md
├── iterative-retrieval/SKILL.md
├── strategic-compact/SKILL.md
└── instincts/SKILL.md
skills/<skill>/SKILL.md         # copia publicada, idéntica byte a byte
```

## Estructura obligatoria de cada SKILL.md

1. Un frontmatter con `name: <directorio>` y `description:` (≤ 300 caracteres, con disparadores en español).
2. Una sección **Capacidades**, en este orden de preferencia: tools MCP de gomemory, luego CLI `mem …`, y por último el procedimiento manual sin ninguna de las dos. Cuando no existe capacidad MCP para una operación (por ejemplo, instincts list/confirm/reject o `rules --check`), la CLI es la vía principal y la skill lo dice explícitamente. La skill tiene que seguir siendo completa en los tres casos (patrón de defensive-qa-audit).
3. Una sección **Procedimiento** con pasos numerados y un criterio de salida verificable.
4. Una tabla **Por harness** (`| Acción | Claude Code | Codex | OpenCode |`) cuando hace falta un comando nativo. Es el único sitio donde se admiten comandos `/…` exclusivos de un harness.
5. Al final, la línea `Inspirada en <skill ECC> de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.`

## Contenido mínimo por skill

| Skill | Tools MCP que DEBE nombrar | Criterio de salida |
|---|---|---|
| verify-loop | `search_memories`, `save_memory`, `review_start` (+ CLI `mem expand rules --check` sobre el diff) | veredicto PASS/FAIL guardado (`type=decision`, o `bugfix` si se encontró algo) |
| search-first | `search_memories`, `search_code`, `get_symbol` | decisión Adopt/Extend/Build guardada con `save_memory type=decision` |
| iterative-retrieval | `pack_build`, `pack_retrieve`, `octopus_route_task` | paquete acotado (como mucho 3 ciclos) entregado al subagente |
| strategic-compact | `save_session_summary` | resumen guardado antes de compactar, o la decisión explícita de no compactar |
| instincts | `save_memory` | cada instinct revisado queda confirmado (`mem expand instincts confirm`) o rechazado |

## Prueba de contrato (`tests/contract/expand_skills_contract_test.go`)

Falla si, en alguna skill:

- `name` ≠ nombre del directorio;
- aparece un identificador con forma de tool de gomemory (`\b[a-z]+(_[a-z]+)+\b` en contexto de código, entre backticks) que no está en `domain.AllMCPTools()` y no figura en una lista de excepciones explícita de la prueba;
- aparece `npm`, `npx`, `pnpm`, `yarn`, `~/.claude`, `~/.codex` o `~/.config/opencode` en cualquier sitio, o `/compact`, `/context` o `/status` fuera de la tabla «Por harness»;
- falta la línea de atribución, o falta `NOTICE`;
- la copia publicada difiere de la canónica (`TestSkillSourcesInSync` ampliada).

## Instalación

`setup.InstallExpandSkills(map[name]body)` hace un bucle sobre `InstallAgentSkill(name, body)` sin filtro de agente (único dueño). Se llama en `runGlobalScopeSetup` después de socratica y, con alcance proyecto, desde `mem install` (FR-019). Destinos: `~/.claude/skills`, `~/.codex/skills` y `~/.config/opencode/skills` (el directorio global que OpenCode 2.x descubre siempre), solo si el directorio base del agente existe. La copia antigua en `~/.opencode/skills/<name>` se retira tras escribir el destino, solo si es idéntica a la instalada y su directorio no contiene nada más; nunca se borra en recursivo.
