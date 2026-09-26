# Contrato del hook `session-start`: avisos visibles para la persona

El hook sigue sin bloquear nunca, sin red y con código 0 (FR-004, FR-029).

## Entradas nuevas

- **Aviso de versión** (FR-030): `latest` de `update-check.json` es más nueva que `version.Version`, el aviso no está desactivado y `.memory/.update-notice` no registra esta versión en esta sesión.
- **Aviso de retirada** (FR-004a): en esta ejecución se retiró una copia local.

Si hay varios, se unen en líneas separadas en el orden retirada → versión.

## Efectos secundarios, en este orden

1. Retirada de la copia local, si procede (best-effort, R2).
2. Si la caché de versiones está vencida y el aviso no está desactivado, se lanza `mem update-check` desacoplado. **No se espera.**
3. Se construye el contexto (sin cambios).
4. Se emite la salida según el dialecto.
5. Si se emitió un aviso de versión, se escribe `.memory/.update-notice`.

## Salida por dialecto

| Dialecto | Sin aviso | Con aviso |
|---|---|---|
| `claude` | Contexto en texto plano (**igual que hoy**) | `{"systemMessage": "<avisos>", "hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": "<contexto>"}}` |
| `json` (Codex) | Igual que hoy | Lo de hoy más `systemMessage: "<avisos>"` |
| `neutral` / `text` | Igual que hoy | `Informa a la persona: <avisos>` + línea en blanco + contexto |

Textos exactos:
- Versión: `gomemory v2.26.5 disponible (tienes v2.26.4) → mem update`
- Retirada: `Se retiró <ruta>/mem (v2.8.0); ahora se usa el global v2.26.4 (<ruta-global>)`

## Invariantes que se prueban

- Sin avisos, la salida es byte a byte idéntica a la de v2.26.4 en todos los dialectos (las pruebas de contrato existentes no cambian).
- Con `GOMEMORY_NO_UPDATE_CHECK=1`, el hook no lanza ningún proceso y no escribe `update-check.json`.
- Un `update-check.json` corrupto se trata como vencido y no produce ningún aviso.
- El hook termina en el mismo tiempo, con y sin red (SC-006). La prueba usa una API inalcanzable y comprueba que la duración no aumenta.
