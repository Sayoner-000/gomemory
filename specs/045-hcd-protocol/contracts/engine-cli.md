# Contrato: motor `mem hcd` (v2)

**Cubre**: FR-002, FR-009 a FR-020, FR-023. **v1**: `git show a0503b0:specs/045-hcd-protocol/contracts/engine-cli.md`.

## 1. Invocación

```
mem hcd <comando> [unidad] [flags] [--json] [--client terminal|claude|codex|opencode|tui]
```

Sin unidad, cada comando usa la unidad activa del proyecto. Las mutaciones dejan activa la unidad sobre la que actúan.

## 2. Comandos

| Comando | Flags | Escribe | Efecto |
|---|---|---|---|
| `start` | `--kind` `--lane fast\|full` `--title` `--why` `[--intent]` | sí | Crea la unidad y su documento en `hcd/` y la deja activa. Sin `--why` → `validation`. |
| `show`, `next` | — | **no** | Paso, tareas, faltantes, rojo pendiente y siguiente skill. |
| `clarify` | `--ask 'pregunta'` \| `--id Qn --answer 'respuesta'` | sí | Abre o resuelve una aclaración en el documento; la respuesta se guarda como memoria `decision`. |
| `approve` | `--quote 'cita'` | sí | Solo carril `full` y paso `approve`: aprueba la revisión exacta de la definición. |
| `verify` | `--task N[,M]` `[--expect-fail]` `-- 'comando'` | sí | Ejecuta el comando en la raíz y guarda la evidencia con la revisión de trabajo y la huella. |
| `verify` | `--task N` `--manual 'observación'` | sí | Evidencia manual, marcada como tal. |
| `analyze` | — | **no** | Hallazgos de coherencia con severidad `CRITICAL`, `HIGH` o `MEDIUM`. |
| `close` | `--quote 'cita'` | sí | Exige el paso `close`, cero hallazgos `CRITICAL` y una cita válida sobre la revisión de trabajo. |
| `cancel` | `--quote 'motivo'` | sí | Exige una cita no vacía. |
| `promote` | — | sí | De `fast` a `full`: añade Historias y Slices al documento y conserva la evidencia. |

Todo lo que sigue a `--` es el comando de `verify` y se pasa a `sh -c` (`cmd /C` en Windows). Por eso `--json` y `--client` van **antes** de `--`. Una cita válida es `apruebo`, `aprobar` o la revisión de 12 hexadecimales.

## 3. Pasos

| Carril | Secuencia |
|---|---|
| `full` | `define → clarify → approve → slice → tasks → implement → verify → close → closed` |
| `fast` | `define → clarify (si hay aclaraciones abiertas) → tasks → implement → verify → close → closed` |

`next_action.skill` nombra la skill del paso: `hcd-define`, `hcd-clarify`, `hcd-approve`, `hcd-slice`, `hcd-tasks`, `hcd-implement`, `hcd-verify` o `hcd-close`.

## 4. Salida `--json`

```json
{
  "protocol_version": 2, "ok": true,
  "unit_id": "HCD-001", "kind": "feature", "lane": "full", "state": "active",
  "stage": "approve", "doc_path": "hcd/HCD-001-exportar-csv.md",
  "definition_revision": "1a2b3c4d5e6f", "work_revision": "9f8e7d6c5b4a", "approved": false,
  "fingerprint": "a811d1e54b31+e3b0c44298fc",
  "tasks": [{"id": "1", "text": "…", "check": "…", "status": "pending|unverified|failed|verified"}],
  "pending_checks": [{"code": "DOC-MISSING", "field": "Intención", "message": "…"}],
  "red_missing": ["1"],
  "findings": [{"severity": "CRITICAL", "code": "APPROVAL-MISSING", "ref": "1a2b3c4d5e6f", "message": "…"}],
  "next_action": {"skill": "hcd-approve", "command": "approve", "why": "…"},
  "warnings": [],
  "data": {"clarification": "Q1", "evidence": {"id": "…", "tasks": ["1"], "kind": "command", "expect_fail": true, "exit_code": 1, "passed": false, "red_observed": true, "excerpt": "…"}}
}
```

`findings` solo se rellena en `analyze`. Si hay error: `ok: false` y `error: {"code", "message"}`.

## 5. Códigos de salida

| Código | `error.code` |
|---|---|
| 0 | — |
| 1 | `internal` |
| 2 | `usage` |
| 3 | `not_found` |
| 5 | `not_allowed`, `invalid_quote`, `validation` |

## 6. Gates

- **Aprobación** (FR-011): `approve` liga la aprobación a la revisión de la definición (Intención, Reproducción, Alcance, Historias, Aclaraciones). Editarla después vuelve al paso `approve`; añadir slices o tareas no.
- **Evidencia** (FR-014): una tarea está `verified` solo si su evidencia más reciente con la revisión de trabajo y la huella actuales pasó. Una evidencia roja nunca verifica.
- **Rojo** (FR-015): en `fix`, la tarea 1 exige un rojo observado sobre una huella distinta de la de su verde; fuera de git, anterior al verde (aviso en `warnings`). En los demás tipos, solo las tareas que ya registraron un intento rojo.
- **Cierre** (FR-018): paso `close`, cero `CRITICAL` y cita válida. La aprobación guarda revisión de trabajo, huella, origen (`terminal` si `--client terminal`, si no `conversation`), cliente y cita protegida.
- **Huella**: `HEAD` más el contenido de los archivos cambiados y sin rastrear, excluyendo `hcd/`.
