# Quickstart: validación de Socratica (039)

Guía de validación; el detalle de cada contrato está en `contracts/` y `data-model.md`.

## Prerrequisitos

- Go 1.27.x (constitución §1.2), repo en la raíz.
- Banco aislado: `HOME`, `XDG_*`, `GOMEMORY_DATA_HOME` **y `PATH`** apuntando a directorios temporales (lección memoria 438: aislar PATH antes de `mem install/update`).
- Agentes reales solo para la aceptación por agente (paso 6); el resto usa fakes y no hace llamadas de pago (SC-010).

## 1. Unitarias puras (dominio)

```sh
go test ./domain/ -run 'Socratica|Interception|Reduce|ParseGeneration' -count=1
```

Esperado: tablas que cubren captura en reposo; paso de `/`, adjuntos, imagen, ocupado, baja información, flujo pendiente y origen aprobado; `>` eliminado exactamente una vez; intercepción desactivada; duplicado de approve; reanudación de approved sin ack → review recuperable; fallo del modelo que conserva el original; parser con 0, 1–3 y >3 preguntas, opciones <2 y >4, y JSON roto reparable o no reparable.

## 2. Repositorio contra SQLite real

```sh
go test ./adapters/secondary/persistence/ -run Socratica -count=1
```

Esperado: tope de 20 por `(project, client)`, `approve` concurrente con un solo `dispatch_id`, `<private>` ausente al leer y al escribir, y `mem sanitize` encuentra candidatos en las columnas nuevas.

## 3. Instalación idempotente y desinstalación sin rastros

```sh
go test ./adapters/primary/setup/ -run 'Socratica|InstallAgentSkill' -count=1
go test ./tests/contract/ -run 'Uninstall_SistemaSinRastros|SkillSourcesInSync' -count=1
```

Esperado: la segunda instalación no escribe nada, no se crean directorios de agentes ausentes, los envoltorios no contienen el cuerpo de la habilidad y `skills/*/SKILL.md` coincide con `infrastructure/templates/*/SKILL.md`.

## 4. CLI de extremo a extremo con generador falso

```sh
go test ./adapters/primary/cli/ -run 'Socratica' -count=1
```

Esperado: `refine` → Ejecutar (`--print`) emite el prompt una vez y la segunda vez informa «ya enviado»; Editar → Ejecutar; Cancelar repite el original literal; sin generador devuelve código 2 y el original; `on` en un agente no soportado devuelve código 1 sin persistir; `on`/`off` de un agente no altera a otro; `history` aplica el tope de 20.

## 5. TUI

```sh
go test ./adapters/primary/tui/ -run 'Socratica' -count=1
```

Esperado: render en 59, 80 y 120 columnas según `contracts/tui.md`; `View()` repetido no muta el modelo ni el repositorio falso.

## 6. Aceptación por agente (manual, con evidencia)

Para cada agente instalado, en un proyecto de prueba con `mem setup-mcp --scope global --agents all`:

1. Invocar `/socratica arregla el login` (o su alternativa de la matriz). Comprobar que hay preguntas o una revisión y que **no se modificó ningún archivo** (`git status` limpio).
2. Elegir Ejecutar dos veces: la segunda debe responder «ya aprobado» sin volver a trabajar.
3. Repetir con Cancelar: el original aparece literal.
4. `mem socratica on --client <agente> --unverified` (la capacidad aún no está evidenciada), enviar un prompt plano, luego `/help` y luego `> hola`. Solo el primero debe entrar en Socratica.
5. Pasar los 5 prompts de referencia de `evidence/README.md` (SC-005) y comprobar que cada dato ausente queda como `[marcador]`.
6. Desde la CLI, `mem socratica refine --client <agente>` → Ejecutar: el agente se abre con el prompt y, con la intercepción activada, **no** lo recaptura (`GOMEMORY_SOCRATICA_DISPATCH`).
7. Guardar órdenes y salidas en `evidence/<agente>.md` y completar `contracts/compatibility-matrix.md`.

## 7. Validación de la habilidad

```sh
skills-ref validate skills/socratica   # solo si skills-ref está instalado; hoy no lo está → registrar "no ejecutado"
```

## 8. Regresión completa

```sh
go test ./... -count=1
```

Esperado: verde, sin regresiones en MCP, sesiones, distribución de habilidades, hooks ni Octopus (SC-009).
