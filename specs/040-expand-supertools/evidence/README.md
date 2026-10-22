# Evidencias de 040: verificación real

Cada verificación real queda registrada aquí: prueba sobre el binario recompilado y los harnesses reales, no solo con tests unitarios. Las rutas se anonimizan (`<scratch>`, `<home>`) y no se copia nunca una credencial.

## Formato por registro

```markdown
### <Escenario> — <harness>

- Fecha: AAAA-MM-DD
- Harness: <nombre> <versión exacta>
- Binario: `mem version` → <salida>
- Orden ejecutada / petición al agente: <literal>
- Salida literal (recortada a lo relevante):
  ```text
  …
  ```
- Veredicto: PASS | FAIL | EXCEPCIÓN DECLARADA (motivo y enlace a la matriz de docs/expand.md)
```

## Verificaciones previstas (quickstart)

| § | Escenario | Archivo | Tarea |
|---|---|---|---|
| — | Sonda de capacidades de Codex | `codex-probe.md` | T004 ✓ |
| §2–§4 | Instalación limpia, bloqueo destructivo, reglas propias y sugerencia única en 3 harnesses | `us1-destructive.md` | T032 |
| §6–§7 | Interruptor, update, idempotencia y uninstall | `us2-lifecycle.md` | T039 |
| §4, §9 | Sugerencia de flujo y skills presentes (destino de OpenCode: C-002) | `us3-skills-opencode.md` | T053 |
| §8 | `mem doctor --shield` en la máquina de referencia | `us4-shield.md` | T060 |
| §10 | Instincts en dos proyectos | `us5-instincts.md` | T070 |
| §11 | Evolve con TTY | `us6-evolve.md` | T073 |
| §5 | Latencia de los 5 eventos | `latency.md` | T074 |
| — | Revisión final y FR-036 | `final.md` | T079 |
