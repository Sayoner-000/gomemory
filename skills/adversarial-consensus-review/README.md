# Adversarial Consensus Review

Revisión adversarial por consenso con dos revisores independientes de solo lectura.
Agnóstico a agente, framework y proveedor de modelo.

## Instalación

### Con gomemory (recomendado)

```bash
mem setup-mcp --scope global
```

Esto distribuye la habilidad a los tres agentes que exponen directorio de skills:
`.claude/skills/`, `.codex/skills/`, `.opencode/skills/`.

### Manual

Copia `SKILL.md` al directorio de habilidades de tu agente:

**Claude Code:**
```bash
mkdir -p ~/.claude/skills/adversarial-consensus-review
cp SKILL.md ~/.claude/skills/adversarial-consensus-review/
```

**Codex:**
```bash
mkdir -p ~/.codex/skills/adversarial-consensus-review
cp SKILL.md ~/.codex/skills/adversarial-consensus-review/
```

**OpenCode:**
```bash
mkdir -p ~/.opencode/skills/adversarial-consensus-review
cp SKILL.md ~/.opencode/skills/adversarial-consensus-review/
```

### Desde curl (sin gomemory)

```bash
SKILL_DIR=""
case "$(uname -s)" in
  Darwin)
    if [ -d ~/.claude ]; then SKILL_DIR=~/.claude/skills; fi
    if [ -d ~/.codex ]; then SKILL_DIR=${SKILL_DIR:-~/.codex/skills}; fi
    if [ -d ~/.opencode ]; then SKILL_DIR=${SKILL_DIR:-~/.opencode/skills}; fi
    ;;
esac
SKILL_DIR=${SKILL_DIR:-~/.claude/skills}

mkdir -p "$SKILL_DIR/adversarial-consensus-review"
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/skills/adversarial-consensus-review/SKILL.md \
  -o "$SKILL_DIR/adversarial-consensus-review/SKILL.md"

echo "Instalado en $SKILL_DIR/adversarial-consensus-review/SKILL.md"
echo "Reinicia tu agente para que la detecte."
```

## Uso

Una vez instalada, el agente la activa cuando pides:

- "revisión adversarial"
- "review independiente"
- "dual review"
- "consensus review"
- "cross-check"
- "segunda opinión"
- "validación profunda"
- "pre-merge validation"
- "pre-release review"

### Ejemplos

```
Revisa el diff actual con adversarial-consensus-review.
```

```
Usa adversarial-consensus-review sobre esta especificación antes de implementar.
```

```
Revisa la arquitectura con dos revisores independientes y reporta solo defectos
corroborados de alta confianza.
```

```
Implementa el cambio pedido, corre revisión adversarial, corrige defectos HIGH o
CRITICAL confirmados y re-verifícalos antes de dar por terminado.
```

## Protocolo resumido

```
TARGET → FREEZE → REVIEWER A (solo lectura)
                 → REVIEWER B (solo lectura)
                 → CONSENSUS
                 → CONFIRMED / SUSPECT / CONTRADICTION
                 → FIX (solo autorizado)
                 → RE-JUDGMENT (independiente)
                 → VEREDICTO: APPROVED / ESCALATED / INCOMPLETE
```

## Invariantes clave

1. Ambos revisores inspeccionan el mismo target congelado
2. Los revisores son independientes entre sí
3. Los revisores no modifican el target
4. Todo hallazgo accionable requiere evidencia concreta
5. La corrección está acotada a hallazgos confirmados autorizados
6. Las rondas de corrección tienen presupuesto fijo
7. La verificación posterior es independiente de la corrección

## Requisitos

- Un agente que soporte subagentes o sesiones aisladas (parcial o totalmente)
- Un target concreto que revisar (código, specs, planes, configs, etc.)

## Licencia

Apache-2.0
