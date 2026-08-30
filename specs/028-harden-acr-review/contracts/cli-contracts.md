# Contrato — CLI `mem review` (028)

Delta sobre `specs/027-adversarial-consensus-review/contracts/cli-contracts.md`. Lo que
no aparece aquí no cambia. Salida a `stdout`, diagnósticos a `stderr`, código de salida
distinto de cero ante cualquier rechazo.

---

## `mem review --pending` **[nuevo]**

Congela **todo** el trabajo pendiente del proyecto como target de revisión: cambios
preparados, cambios sin preparar y archivos nuevos no ignorados (FR-025).

```
$ mem review --pending
acr_3f9c1a20-…
target_digest: 8b41d2f7…
target_files: 7
independence: full
```

**Comportamiento**

- La lista de rutas se obtiene con `git status --porcelain=v1 -z --untracked-files=all`,
  que respeta `.gitignore` y admite nombres con espacios.
- Las rutas se ordenan antes de hashear, de modo que el digest no depende del orden en
  que git las devuelva.
- Cada archivo contribuye `"pending\x00" + ruta + "\x00" + contenido + "\x00"`; un
  archivo borrado contribuye su ruta con contenido vacío y un marcador de borrado, para
  que borrar cambie la identidad del target.
- Los archivos vacíos y binarios se incluyen: forman parte de lo que hay que revisar.

**Rechazo (FR-026)**

```
$ mem review --pending
error: no hay cambios pendientes que revisar
$ echo $?
1
```

**Por qué existe**: `--diff` usa `git diff --binary`, que no ve los archivos sin
seguimiento. Una revisión de trabajo en curso con archivos recién creados congelaba un
target que no los contenía, y los revisores inspeccionaban menos de lo que se creía.

---

## `mem review --diff` / `--commit` / `--file`

Sin cambios en la resolución del target. Cambia la salida, que ahora imprime la
política efectiva con la que quedó congelada la revisión:

```
$ mem review --diff
acr_3f9c1a20-…
target_digest: e7114e8c…
independence: degraded (same-model)
fix_authorized: true
max_fix_rounds: 2
auto_fix_severities: CRITICAL, HIGH
```

Esos valores salen de la política del proyecto (`review_max_fix_rounds`,
`review_auto_fix_severities`, `review_fix_authorized` en `Settings`) y ya no de
constantes escritas en `start_review.go` (FR-017).

## `--read-only` **[nuevo, aplicable a todos los modos de target]**

```
$ mem review --diff --read-only
acr_3f9c1a20-…
…
fix_authorized: false
```

Inicia una revisión que no admite correcciones (FR-018). Con un hallazgo confirmado
severo, esta revisión finaliza `ESCALATED` en una sola llamada en vez de quedar
bloqueada esperando una corrección que su alcance prohíbe (FR-019).

---

## `mem review show <review-id>`

**Salida ampliada** (FR-023, SC-006): un auditor debe reconstruir el recorrido completo
de cualquier hallazgo con esta sola orden, sin abrir la base de datos.

```
$ mem review show acr_3f9c1a20-…

Revisión acr_3f9c1a20-…            estado: rejudging      veredicto: —
Target   diff working-tree
  original: e7114e8c…
  vigente:  a91f3b02…   (ronda 1)
Política  fix_authorized=true  max_fix_rounds=2  auto_fix=CRITICAL,HIGH
Independencia  full

Revisores
  A  esperado …/…   success   3 hallazgos
  B  esperado …/…   success   2 hallazgos

Hallazgos de consenso
  C-001  CONFIRMED      HIGH    fuentes 12, 27      corregido en ronda 1
         re-juicio  A=RESOLVED  B=—                 agregado: UNRESOLVED
  C-002  CONFIRMED      MEDIUM  fuentes 14, 31      sin corregir
         re-juicio  —                               agregado: UNRESOLVED
  C-003  SUSPECT        LOW     fuente 19
  C-004  INFO           INFO    fuente 22

Correcciones
  ronda 1  e7114e8c… → a91f3b02…   aborda C-001
           rutas: domain/verdict.go, application/usecases/build_consensus.go
           verificación: go test ./domain/... ./application/...

Recuentos
  por clasificación  CONFIRMED 2  SUSPECT 1  CONTRADICTION 0  INFO 1
  por severidad      CRITICAL 0  HIGH 1  MEDIUM 1  LOW 1  INFO 1
  por re-juicio      RESOLVED 0  UNRESOLVED 2  REGRESSED 0  PENDIENTE 1
```

---

## `mem review status [<review-id>]`

Añade a la salida vigente los recuentos por clasificación, por severidad y por estado
de re-juicio (FR-022). Sin `<review-id>` sigue mostrando la revisión activa más
reciente del proyecto.

---

## Estados terminales

Cualquier orden que intente modificar una revisión `approved`, `escalated` o
`incomplete` termina con código 1 y el diagnóstico:

```
error: la revisión está en estado terminal <estado> y no admite cambios
```

Las órdenes de solo lectura (`status`, `show`, `history`) siguen funcionando sobre
revisiones terminales: el ledger se conserva íntegro y consultable (FR-028).
