# US3 — destino de skills en OpenCode (cierre de C-002 del ACR)

### Descubrimiento de skills — OpenCode

- Fecha: 2026-10-21
- Harness: OpenCode v2.0.32 (`opencode --version`)
- Binario: árbol de trabajo de 040 recompilado.

**Qué directorios lee OpenCode.** El bundle (`ConfigSkillPlugin`) añade `<dir>/skill` y `<dir>/skills` por cada entrada `directory` de las fuentes de configuración, más `~/.claude/skills` y `~/.agents/skills` por compatibilidad. Esas fuentes son las que lista `opencode debug config`:

```text
# cwd = proyecto bajo el home (<home>/home/rcw/gomemory)
document <home>/.config/opencode/opencode.json
directory <home>/.config/opencode
directory <home>/.opencode
directory <home>/home/rcw/gomemory/.opencode

# cwd = directorio fuera del home (<scratch>/ocwork)
document <home>/.config/opencode/opencode.json
directory <home>/.config/opencode
```

`~/.opencode` solo aparece por el walk-up desde un proyecto situado bajo el home. El único directorio global que se lee siempre es `~/.config/opencode`.

**Antes del cierre.** El destino era `~/.opencode/skills` y solo se escribía si `~/.opencode` existía. Con un HOME que solo tiene `.config/opencode`, `mem setup-mcp --scope global` no escribía ninguna skill (reproducción del revisor B).

**Después del cierre.** Con el mismo HOME aislado (solo `.config/opencode`), `mem setup-mcp --scope global --agents opencode` termina con exit 0 y escribe:

```text
~/.config/opencode/skills/adversarial-consensus-review/SKILL.md
~/.config/opencode/skills/atomic-decomposition/SKILL.md
~/.config/opencode/skills/constitution/SKILL.md
~/.config/opencode/skills/defensive-qa-audit/SKILL.md
~/.config/opencode/skills/instincts/SKILL.md
~/.config/opencode/skills/iterative-retrieval/SKILL.md
~/.config/opencode/skills/search-first/SKILL.md
~/.config/opencode/skills/socratica/SKILL.md
~/.config/opencode/skills/strategic-compact/SKILL.md
~/.config/opencode/skills/verify-loop/SKILL.md
```

No crea `~/.opencode`. Al reinstalar, la copia que una versión anterior dejó en `~/.opencode/skills/<name>` se retira para que OpenCode no descubra dos copias bajo el home, pero solo tras escribir el destino y solo si es idéntica y está sola en su directorio (B-002 de la 3.ª ronda del ACR); una copia personalizada o con auxiliares se conserva. La desinstalación de sistema limpia ambas rutas.

**Límite de la verificación.** El endpoint `skill.list` de `opencode api --standalone` devuelve `[]` incluso en el HOME real con skills instaladas, así que no sirve para listar el descubrimiento local. La conclusión se apoya en el código del bundle y en las fuentes de `opencode debug config`. No se lanzó una sesión con modelo.

- Veredicto: PASS (destino corregido a `~/.config/opencode/skills`; pruebas `TestInstallAgentSkill_OpenCodeDestinoGlobalYMigracion` y `TestInstall_AlcanceProyectoEntregaSkillsDeExpand`).
