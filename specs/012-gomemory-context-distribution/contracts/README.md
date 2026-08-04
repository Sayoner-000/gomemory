# Contratos — spec 012

- **`install-speckit-extension.md`**: contrato de la función Go
  `setup.InstallSpeckitExtension`.
- **`claude-artifact/SKILL.md`**: fixture — el artefacto REAL que `specify
  extension add` genera para Claude Code a partir de
  `.specify/extensions/gomemory-context/commands/speckit.gomemory-context.update.md`.
  Regenerado y verificado en un proyecto de prueba aislado para esta spec
  (idéntico byte a byte al que ya vive en `.claude/skills/
  speckit-gomemory-context-update/SKILL.md` de este mismo repo, generado
  durante la spec 011). `/speckit-tasks` debe copiar este archivo tal cual
  a `infrastructure/templates/gomemory-context/claude/
  speckit-gomemory-context-update/SKILL.md`.
- **`opencode-artifact/speckit.gomemory-context.update.md`**: fixture — el
  artefacto REAL que `specify extension add` genera para OpenCode,
  regenerado igual que el de Claude en un proyecto de prueba con
  `--integration opencode`. `/speckit-tasks` debe copiarlo tal cual a
  `infrastructure/templates/gomemory-context/opencode/
  speckit.gomemory-context.update.md`.

Ambos fixtures existen para no reinventar el formato de cada agente a
mano — ver `research.md` #1 y #2.
