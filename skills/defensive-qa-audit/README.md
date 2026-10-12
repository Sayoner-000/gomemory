# Defensive QA Audit

Auditoría defensiva multilenguaje (Go, Python, JS/TS, Java, PHP, Rust, Shell, SQL,
CI, HTML/CSS) centrada en programación defensiva y prevención de fugas: PII,
credenciales, recursos, memoria, XSS y errores expuestos. Cada hallazgo exige
evidencia reproducible con datos sintéticos y una propuesta de cierre verificable.

Funciona en cualquier agente. Si el agente tiene conectada la memoria de gomemory,
además consulta auditorías previas, usa el grafo de código para el radio de impacto
y guarda los hallazgos relevantes como memoria del proyecto.

## Instalación

### Con gomemory (recomendado)

```bash
mem setup-mcp --scope global
```

`mem install` con alcance global y `mem update` la instalan y la mantienen al día.
Se distribuye a los agentes que exponen directorio de skills y que ya existen en tu
equipo: `~/.claude/skills/`, `~/.codex/skills/`, `~/.opencode/skills/`. Con alcance
de proyecto no se escribe, igual que `adversarial-consensus-review`.

### Manual

```bash
mkdir -p ~/.claude/skills/defensive-qa-audit
cp SKILL.md ~/.claude/skills/defensive-qa-audit/
```

Sustituye `~/.claude` por `~/.codex` u `~/.opencode` según tu agente.

### Desde curl (sin gomemory)

```bash
SKILL_DIR=~/.claude/skills   # o ~/.codex/skills, ~/.opencode/skills
mkdir -p "$SKILL_DIR/defensive-qa-audit"
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/skills/defensive-qa-audit/SKILL.md \
  -o "$SKILL_DIR/defensive-qa-audit/SKILL.md"
```

Reinicia tu agente para que la detecte.

## Uso

El agente la activa cuando pides una "auditoría defensiva", "QA de seguridad",
"revisión de fugas", "hardening" o la invocas por nombre (`/defensive-qa-audit`).

```
Audita el diff actual con defensive-qa-audit.
```

```
Haz una auditoría defensiva de adapters/ priorizando persistencia y bordes de entrada.
```

```
Audita el repositorio con defensive-qa-audit y pasa los hallazgos altos a
adversarial-consensus-review antes de corregir.
```

## Flujo resumido

```
ALCANCE + SHA → CONTEXTO PREVIO (memoria / grafo, opcional)
             → INSPECCIÓN (principios + matriz por lenguaje)
             → REPRODUCCIÓN con datos sintéticos → CONFIRMADO / SOSPECHOSO
             → COMPROBACIONES MECÁNICAS (vet, race, audit, linters)
             → INFORME (cobertura, H-NN, cierre propuesto y su validación)
             → MEMORIA / docs/audits / ACR (opcional)
```

## Licencia

Apache-2.0
