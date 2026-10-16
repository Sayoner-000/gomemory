# Auditoría defensiva 2026-10-16 — cierre

Revisión auditada: `ccd0e7e` con la feature 039 (Socratica) sin commit. Seis hallazgos: 1 alta, 2 medias, 3 bajas. Cada cierre lleva una prueba que falla sin él.

| ID | Severidad | Cierre | Prueba |
|---|---|---|---|
| H-01 `sessions.last_prompt` fuera de `mem sanitize` | Alta | Categoría `session_prompt` → `last_prompt` en candidatos y `Apply` (condicionado al valor leído y al proyecto). La lectura ya depuraba (`activeSessionLastPrompt` aplica `ProtectText`). | `TestSanitize_CubreLastPrompt` (marcador sintético, otro proyecto intacto) |
| H-02 mutaciones Socratica con éxito falso | Media | `ReleaseDispatch`, `MarkDispatched`, `ForceResend`, `DeleteFlow` y `RedactFlow` comprueban filas afectadas y devuelven `ports.ErrSocraticaNoMatch`. | `TestSocraticaMutations_SinFilaNoEsExito`; la liberación con propietario ajeno pasa a informar `ErrSocraticaNoMatch` |
| H-03 stdin sin acotar y lectura corta | Media | `refine -` lee con `readBounded` (tope 1 MiB, `errPayloadTooLarge`); la cola del transcript usa `io.ReadFull` (`tailOf`) y propaga errores. | `TestSocraticaRefine_StdinAcotado`, `TestTailOf_LecturaCorta` (`iotest.OneByteReader`) |
| H-04 texto libre de `reviews` fuera del ledger | Baja | Verificado: `independence_reason` solo toma `same-model` o vacío y `target_digest` es un hash. `target_scope` sí es texto libre del llamador: se protege por elemento al escribir y al leer y entra en el ledger (`review_target_scope`). | `TestSanitize_CubreTargetScope` |
| H-05 permisos de WAL/SHM y ajustes | Baja | `mem.db` se crea con 0600 antes de abrir (SQLite hereda el modo en `-wal`/`-shm`) y los tres se endurecen al abrir bases antiguas; `settings.json` se escribe y endurece a 0600. Los `.mcp.json` del repositorio siguen legibles a propósito. | `TestPermisos_BaseYAjustesPrivados` (umask 022) |
| H-06 lock, procedencia e instalador | Baja | Lock: apertura con `O_NOFOLLOW` y comprobación sobre el descriptor abierto; `Remove` solo borra si obtiene el lock. Instalador: `verifyChecksum` exige una única entrada de dos campos y 64 hex, como `install.sh`, y compara en tiempo constante. | `TestSocraticaLock_NoSigueEnlaces`, `TestSocraticaLock_RemoveNoBorraUnLockRetenido`, `checksum exige una sola entrada…` |

## Decisiones sin cambio de código

- **`>` como escape**: es una vía deliberada para que la persona salte la intercepción, no una frontera de seguridad. El origen «aprobado» no se falsifica escribiendo: exige un `dispatch_id` real, el hash del texto aprobado y se consume una vez. Documentado en `docs/socratica.md`.
- **`curl | sh` sin firma independiente**: riesgo residual aceptado y ya declarado en los scripts; la verificación SHA-256 se mantiene. Firmas (Sigstore) quedan como evaluación futura.
- **Cancelación en la CLI**: no se propaga Ctrl-C al contexto de `Execute`. El agente lanzado recibe la misma señal (por ejemplo, para interrumpir su turno) y cancelar el contexto haría fallar el ack y dejaría el flujo incierto sin motivo. La generación ya está acotada por `socratica_generator_timeout_seconds`.
- **Windows**: no hay `O_NOFOLLOW`; se mantienen `Lstat` y la comprobación sobre el descriptor, y crear enlaces exige privilegios elevados.

## Validación

`go vet ./...` (también con `GOOS=windows`), `go test ./... -count=1` en verde, carreras dirigidas en verde, plugin 46/46, instalador 21/21 con `tsc --noEmit`, cobertura combinada 80,4 % (sin bajar).
