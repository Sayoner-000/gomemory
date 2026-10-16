# Socratica

Refinamiento de prompts y escrutinio socrático con aprobación explícita. Tiene dos
modos:

- `/socratica [texto]`: convierte un prompt en bruto en uno fiel y ejecutable. Hace
  como máximo tres preguntas con opciones y propone una reescritura que conserva la
  intención, marca con `[marcador]` lo que falta y lista lo que añadió.
- `/socratic-q [texto]`: examina una propuesta. Reformula en términos neutrales,
  separa hechos de hipótesis y ofrece una síntesis equilibrada con el mejor argumento
  a favor y la objeción más fuerte.

Nada se ejecuta sin que la persona lo apruebe. Cancelar devuelve el original intacto.

Funciona en cualquier agente. Si el agente tiene conectada la memoria de gomemory,
la herramienta `socratica_flow` guarda el estado del flujo, evita envíos duplicados
y conserva un historial acotado. Sin gomemory, el mismo flujo se sigue en texto,
sin historial.

## Instalación

### Con gomemory (recomendado)

```bash
mem setup-mcp --scope global
```

`mem install` con alcance global y `mem update` la instalan y la mantienen al día en
los agentes que exponen directorio de skills y que ya existen en tu equipo:
`~/.claude/skills/`, `~/.codex/skills/`, `~/.opencode/skills/`. Además escribe
envoltorios finos de comando para `/socratic-q` en Claude Code y para `/socratica` y
`/socratic-q` en OpenCode; los envoltorios solo nombran la habilidad y el modo.

### Manual

```bash
mkdir -p ~/.claude/skills/socratica
cp SKILL.md ~/.claude/skills/socratica/
```

Sustituye `~/.claude` por `~/.codex` u `~/.opencode` según tu agente.

### Desde curl (sin gomemory)

```bash
SKILL_DIR=~/.claude/skills   # o ~/.codex/skills, ~/.opencode/skills
mkdir -p "$SKILL_DIR/socratica"
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/skills/socratica/SKILL.md \
  -o "$SKILL_DIR/socratica/SKILL.md"
```

## Intercepción automática

Está desactivada por defecto y solo se activa desde la línea de comandos o la TUI
(`mem socratica on --client <agente>`), nunca desde el propio agente. Consulta
`docs/socratica.md` para la matriz de compatibilidad por agente.
