# Auditoría defensiva multilenguaje — gomemory

Fecha: 2026-10-11. Revisión: `30528515597292ecfcd42c7c08283cae0fbbbd12`.
Perfil aplicado: `multi_language_defensive_qa_auditor`, proporcionado por el usuario.
Apoyo metodológico: `code-review-skill`.

### 1. Resumen de Salud del Código

- **Lenguajes / Tecnologías Detectadas:** Go, SQLite, MCP por stdio, CLI/TUI, TypeScript/JavaScript, Node.js, Python, Bash, PowerShell y YAML de CI. Java aparece únicamente como corpus de pruebas. No se encontró frontend HTML/CSS ni producción Java, PHP o Rust.
- **Nivel de Riesgo General:** **Alto**.
- **Evaluación Breve:** El proyecto tiene transacciones, consultas parametrizadas, redacción central de memorias, directorios privados, checksums en Go/TypeScript y pruebas de contrato. Sin embargo, la redacción no alcanza varias tablas auxiliares y el parser de bloques privados falla con anidamiento. Estos defectos permiten guardar y reemitir contenido que se marcó explícitamente como privado.
- También se confirmaron estado mutable compartido en el constructor de contexto, una ventana sin ejecutable durante update, dos defectos del adaptador OpenCode v2, éxito falso de wrap y verificación de integridad ausente en los instaladores de scripts. No se confirmó una vulnerabilidad crítica de ejecución remota, SQL injection o XSS.

**Resultado:** 9 hallazgos: 2 altos, 6 medios y 1 bajo. Esta es una inspección dirigida de superficies y flujos de riesgo, con búsquedas globales y reproducciones; no equivale a revisar línea por línea los 1.106 archivos ni a garantizar ausencia de fugas.

**Cobertura y validación**

| Comprobación | Resultado |
|---|---|
| Grafo externo, llamadores y fuente actual | CLI/MCP/hooks, contexto, persistencia, compresión, proveedores externos, instaladores y plugin |
| `go test ./...` | Pasa: 18 paquetes con pruebas, incluidos contrato e integración |
| `go vet ./...` | Pasa |
| `go test -race` en CLI, consola, proveedores de grafo, persistencia, compresión nativa y casos de uso | Pasa con las pruebas existentes |
| Reproducción concurrente adicional de Builder | **Falla: 2 carreras detectadas** |
| Instalador: `npm run typecheck` y `npm test` | Pasa: 20/20 pruebas |
| Plugin: `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | **22/24**; dos expectativas desactualizadas, H-09 |
| `govulncheck` | Sin vulnerabilidades encontradas |
| `npm audit --package-lock-only --omit=dev` | 0 avisos en dependencias de producción; no incluye devDependencies |
| Bash: `bash -n scripts/*.sh` | Pasa |
| Python 3.12: compilación sintáctica sin ejecutar scripts | 4/4 scripts pasan |
| PowerShell | Inspección estática; no se dispone de `pwsh` |
| Reproducciones de propuestas | Copia temporal del repositorio; producción no modificada |

Entorno: macOS arm64, Go 1.27.2, Node 26.11.1. El `python3` del sistema es 3.9.6: no puede analizar el f-string de `acr-smoke.py` que requiere la sintaxis admitida por Python 3.12. Con Python 3.12 pasa; corresponde explicitar la versión mínima.

Las reproducciones usan marcadores y credenciales sintéticas. El binario probado se compiló desde esta revisión; no se reemplazó el binario instalado ni se utilizó el almacén real.

### 2. Hallazgos y Fugas Detectadas

| ID | Severidad | Problema |
|---|---|---|
| H-01 | Alta | Resúmenes, relaciones y errores de canal omiten la redacción |
| H-02 | Alta | Bloques privados anidados/incompletos fallan abiertos |
| H-03 | Media | Builder acumula métricas y presenta carreras entre llamadas |
| H-04 | Media | Update retira el ejecutable antes de preparar su reemplazo |
| H-05 | Media | OpenCode v2 descarta el texto comprimido |
| H-06 | Media | Setup fallido deja suscripción y hooks registrados |
| H-07 | Media | Wrap devuelve éxito cuando no puede iniciar el comando |
| H-08 | Media | Instaladores Bash/PowerShell omiten checksums |
| H-09 | Baja | Pruebas OpenCode usan una firma de llamada obsoleta |

Las correcciones siguientes son propuestas. Se validaron las indicadas en temporales; no se aplicaron al código de producción.

#### H-01 — Redacción incompleta en persistencia y consola

- **Categoría:** Fuga de datos / credenciales.
- **Severidad:** Alta.
- **Ubicación:** [adapters/secondary/persistence/session.go:67](/Users/josegomezj/home/rcw/gomemory/adapters/secondary/persistence/session.go:67), [adapters/secondary/persistence/relation.go:10](/Users/josegomezj/home/rcw/gomemory/adapters/secondary/persistence/relation.go:10), [adapters/secondary/persistence/channel_activity.go:55](/Users/josegomezj/home/rcw/gomemory/adapters/secondary/persistence/channel_activity.go:55) y [adapters/primary/cli/cmd_session.go:62](/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_session.go:62).
- **Análisis de Vulnerabilidad:** `EndSession`, `UpdateSessionSummary`, las tres escrituras de relaciones y `RecordChannelError` guardan texto crudo. MCP y hooks alcanzan estas rutas. `session end -s` además imprime el resumen original; `Builder.Build` incluye resúmenes cerrados en el contexto. Una protección en `InsertMemory` no cubre estas otras tablas.
- **Evidencia:** El binario real guardó un marcador dentro de un bloque privado y un token sintético mediante `hook compact-summary` y `hook channel-error`. `session end` y `context` reemitieron el token. Las APIs reales `InsertRelation`, `UpdateRelation` e `ImportRelation` también lo conservaron.
- **Código Corregido (Seguro y Defensivo):** Añadir el helper en persistencia y reemplazar estas funciones. Los imports correspondientes siguen siendo `database/sql`, `fmt` y `mem/domain`.

```go
func redactPersistedText(s string) string {
 return domain.RedactSecrets(domain.RedactPrivate(s))
}

func EndSession(db *sql.DB, id, summary string) error {
	summary = redactPersistedText(summary)
	res, err := db.Exec(
		`UPDATE sessions SET ended_at = `+Now+`, summary = ? WHERE id = ? AND ended_at IS NULL`,
		summary, id,
	)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %s not found or already ended", id)
	}
	return nil
}

func UpdateSessionSummary(db *sql.DB, id, summary string) error {
	summary = redactPersistedText(summary)
	res, err := db.Exec(
		`UPDATE sessions SET summary = ? WHERE id = ? AND ended_at IS NULL`,
		summary, id,
	)
	if err != nil {
		return fmt.Errorf("update session summary: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %s not found or already ended", id)
	}
	return nil
}

func RecordChannelError(db *sql.DB, project, agent, scope, kind, msg string) error {
	msg = redactPersistedText(msg)
	if project == "" || agent == "" || kind == "" {
		return nil
	}
	_, err := db.Exec(
		`INSERT INTO channel_activity (project, agent, scope, kind, last_error, last_error_at)
		 VALUES (?, ?, ?, ?, ?, `+Now+`)
		 ON CONFLICT(project, agent, scope, kind) DO UPDATE SET
		   last_error = excluded.last_error,
		   last_error_at = excluded.last_error_at`,
		project, agent, scope, kind, msg)
	if err != nil {
		return fmt.Errorf("registrar fallo de canal: %w", err)
	}
	return nil
}

func InsertRelation(db *sql.DB, r *domain.Relation) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, `+Now+`)`,
		r.Project, r.MemoryIDA, r.MemoryIDB, string(r.Relation), r.Confidence, redactPersistedText(r.Reasoning),
	)
	if err != nil {
		return 0, fmt.Errorf("insert relation: %w", err)
	}
	return res.LastInsertId()
}

func UpdateRelation(db *sql.DB, id int64, relation domain.RelationType, confidence float64, reasoning string) error {
	res, err := db.Exec(
		`UPDATE memory_relations SET relation = ?, confidence = ?, reasoning = ? WHERE id = ?`,
		string(relation), confidence, redactPersistedText(reasoning), id,
	)
	if err != nil {
		return fmt.Errorf("update relation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("relation %d not found", id)
	}
	return nil
}

func ImportRelation(db *sql.DB, r *domain.Relation) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?,''), `+Now+`))`,
		r.Project, r.MemoryIDA, r.MemoryIDB, string(r.Relation), r.Confidence, redactPersistedText(r.Reasoning), r.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("import relation: %w", err)
	}
	return res.LastInsertId()
}
```

Antes de persistir e imprimir `finalSummary` en `cmdSessionEnd`, añadir `mem/domain` a sus imports y aplicar:

```go
finalSummary = domain.RedactSecrets(domain.RedactPrivate(finalSummary))
```

- **Validación de la propuesta:** La copia temporal pasa persistencia; las reproducciones dejan de encontrar el token en las tres escrituras de relaciones, resúmenes, errores y salidas de consola/contexto.
- **Cierre:** Probar también las herramientas MCP `end_session` y `save_session_summary`, importación de relaciones y exportación. La protección de nuevas escrituras no limpia filas históricas: diseñar una sanitización explícita y revisable de resúmenes, razonamientos y diagnósticos ya almacenados, con prueba de las salidas posteriores y tratamiento de copias/WAL.

#### H-02 — Bloques privados anidados o sin cierre

- **Categoría:** Fuga de datos / credenciales.
- **Severidad:** Alta.
- **Ubicación:** [domain/redact.go:5](/Users/josegomezj/home/rcw/gomemory/domain/redact.go:5).
- **Análisis de Vulnerabilidad:** La expresión regular termina en el primer cierre y no reconoce profundidad. En `<private>outer <private>inner</private>secreto</private>`, el texto posterior al cierre interior queda visible. Un bloque sin cierre tampoco se elimina. Esto afecta memorias, prompts y `IsPrivateContent`: la compresión puede dejar de clasificar un bloque incompleto como privado.
- **Evidencia:** `mem save` devolvió éxito y el marcador situado entre ambos cierres quedó en SQLite.
- **Código Corregido (Seguro y Defensivo):** Sustituir `privateBlock` y `RedactPrivate`; añadir `strings` al import existente. El contenido de un bloque sin cierre se descarta hasta el final.

```go
var privateTag = regexp.MustCompile(`(?i)</?private>`)

func RedactPrivate(s string) string {
 var out strings.Builder
 depth, cursor := 0, 0
 for _, match := range privateTag.FindAllStringIndex(s, -1) {
  if depth == 0 {
   out.WriteString(s[cursor:match[0]])
  }
  tag := s[match[0]:match[1]]
  if strings.EqualFold(tag, "<private>") {
   depth++
  } else if depth > 0 {
   depth--
  }
  cursor = match[1]
 }
 if depth == 0 {
  out.WriteString(s[cursor:])
 }
 return out.String()
}
```

- **Validación de la propuesta:** Suite domain y casos adicionales de anidamiento, apertura sin cierre, mayúsculas y Unicode pasan. La reproducción de guardado deja de persistir el marcador.
- **Cierre:** Añadir regresiones permanentes en redacción, prompts y compresión; comprobar que el texto público se conserva y que no se generan originales recuperables para contenido privado incompleto.

#### H-03 — Estado de construcción compartido

- **Categoría:** Error lógico / concurrencia / consumo de memoria.
- **Severidad:** Media.
- **Ubicación:** [application/usecases/build_context.go:287](/Users/josegomezj/home/rcw/gomemory/application/usecases/build_context.go:287); reutilización en [adapters/primary/cli/cmd_mcp.go:415](/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp.go:415) y [infrastructure/container.go:1](/Users/josegomezj/home/rcw/gomemory/infrastructure/container.go:1).
- **Análisis de Vulnerabilidad:** `discardedChars` se acumula en el receptor y no se reinicia en `Build`. El container conserva el mismo Builder para MCP. Las llamadas repetidas exageran la línea base y el ahorro; las concurrentes escriben el mismo contador sin sincronización. La asignación `strings.Repeat` usada para contar tokens crece con ese acumulado; es crecimiento de trabajo temporal, no una fuga permanente de memoria demostrada.
- **Evidencia:** Con una memoria idéntica, tres llamadas dieron `baseline=6073,12073,18073` y `emitted=73`. Una reproducción con ocho goroutines confirmó dos carreras en `acota`.
- **Código Corregido (Seguro y Defensivo):** Renombrar el cuerpo actual de `Build` a `build` y añadir este wrapper. La configuración del Builder se mantiene inmutable durante el servicio; cada llamada recibe su propio contador.

```go
func (b *Builder) Build() (string, error) {
 local := *b
 local.discardedChars = 0
 return local.build()
}
```

- **Validación de la propuesta:** Domain, casos de uso y CLI pasan. La misma reproducción entrega `6073/73` tres veces y termina sin avisos con `-race`.
- **Cierre:** Pruebas permanentes de tres construcciones secuenciales y construcciones concurrentes; ejecutar la variante de concurrencia mediante llamadas MCP solapadas. Si en el futuro cambia la configuración en vivo, deberá usarse un snapshot sincronizado.

#### H-04 — Actualización Unix sin reemplazo atómico

- **Categoría:** Error lógico / resiliencia.
- **Severidad:** Media.
- **Ubicación:** [adapters/primary/cli/cmd_update.go:446](/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_update.go:446).
- **Análisis de Vulnerabilidad:** `replaceSelf` renombra primero el ejecutable a `.old` y después copia al nombre público. Durante ese intervalo no existe `mem`; durante la copia puede existir incompleto. Una interrupción puede exigir recuperación manual. El rollback de un error devuelto por copia no protege frente a la terminación del proceso.
- **Evidencia:** Las funciones exactas, copiadas a una prueba temporal, dejaron la ruta del ejecutable ausente mientras la copia estaba bloqueada. El FIFO es una técnica de observación de la secuencia; la copia habitual del binario descargado es un archivo regular.
- **Código Corregido (Seguro y Defensivo):** Preparar y sincronizar un archivo en el mismo directorio, y renombrarlo cuando esté listo. Este cambio conserva la política Windows actual.

```go
func replaceSelf(currentPath, newPath string) error {
 if runtime.GOOS == "windows" {
  finalPath := currentPath + ".new"
  if err := copyFile(newPath, finalPath); err != nil {
   return err
  }
  return fmt.Errorf(
   "windows bloquea el binario en ejecución. El nuevo binario quedó en %s.\n"+
    "Cierra este proceso y ejecuta:\n"+
    "  move /Y \"%s\" \"%s\"",
   finalPath, finalPath, currentPath,
  )
 }
 in, err := os.Open(newPath)
 if err != nil {
  return fmt.Errorf("abrir binario nuevo: %w", err)
 }
 defer func() { _ = in.Close() }()
 staged, err := os.CreateTemp(filepath.Dir(currentPath), ".mem-update-*")
 if err != nil {
  return fmt.Errorf("preparar binario nuevo: %w", err)
 }
 stagedPath := staged.Name()
 defer func() { _ = os.Remove(stagedPath) }()
 defer func() { _ = staged.Close() }()
 if err := staged.Chmod(0o755); err != nil {
  return fmt.Errorf("permisos de binario nuevo: %w", err)
 }
 if _, err := io.Copy(staged, in); err != nil {
  return fmt.Errorf("copiar binario nuevo: %w", err)
 }
 if err := staged.Sync(); err != nil {
  return fmt.Errorf("sincronizar binario nuevo: %w", err)
 }
 if err := staged.Close(); err != nil {
  return fmt.Errorf("cerrar binario nuevo: %w", err)
 }
 if err := os.Rename(stagedPath, currentPath); err != nil {
  return fmt.Errorf("reemplazar binario actual: %w", err)
 }
 return nil
}
```

- **Validación de la propuesta:** Regresiones CLI pasan. Una prueba temporal conserva el ejecutable anterior durante la copia parcial, publica el nuevo con modo 0755 al finalizar y mantiene el instalado cuando falta la fuente.
- **Cierre:** Probar cancelación antes del rename, permisos insuficientes y lanzamiento concurrente de `mem`. Este reemplazo atómico protege visibilidad; una garantía de durabilidad frente a pérdida de energía requiere decidir y validar además la sincronización del directorio según plataforma.

#### H-05 — Compresión sin efecto en OpenCode v2

- **Categoría:** Error lógico.
- **Severidad:** Media.
- **Ubicación:** [infrastructure/plugin/opencode/gomemory.ts:739](/Users/josegomezj/home/rcw/gomemory/infrastructure/plugin/opencode/gomemory.ts:739).
- **Análisis de Vulnerabilidad:** El adaptador pasa un objeto temporal al hook v1. Este modifica `output.output`, pero el adaptador no copia el cambio a `ev.result`. Se consume trabajo de compresión sin reducir el resultado entregado por esa ruta.
- **Evidencia:** Con compresión habilitada y resultado de 12.000 caracteres, se llamó a `hook tool-output opencode` y `ev.result.content` conservó todo el texto original.
- **Código Corregido (Seguro y Defensivo):** Añadir el helper y sustituir el bloque de registro. Mantener las partes no textuales y los metadatos de la primera parte textual.

```typescript
const applyResultText = (result: any, text: string): void => {
  if (!result || typeof result !== "object") return;
  if (typeof result.content === "string") {
    result.content = text;
  } else if (Array.isArray(result.content)) {
    let replaced = false;
    result.content = result.content.flatMap((part: any) => {
      if (part?.type !== "text") return [part];
      if (replaced) return [];
      replaced = true;
      return [{ ...part, text }];
    });
  } else if (typeof result.output === "string") {
    result.output = text;
  }
};

  // v2:tool.execute.after
  registrations.push(
    await ctx.tool.hook("execute.after", (ev: any) =>
      safe(async () => {
        if (ev.status !== "completed" || !(await ownsSession(ev.sessionID))) return;
        const original = resultText(ev.result);
        const output = { output: original };
        await hooks["tool.execute.after"](
          { tool: v1ToolName(ev.tool), sessionID: ev.sessionID, callID: ev.id },
          output,
        );
        if (output.output !== original) applyResultText(ev.result, output.output);
      }),
    ),
  );
```

- **Validación de la propuesta:** En el mock que ejecuta el módulo TS actual, la versión propuesta refleja el texto comprimido en el resultado.
- **Cierre:** Pruebas de contenido string, arreglo con texto/imágenes, output string, salida sin ganancia y compresión desactivada. Verificar en OpenCode real que el contrato del hook permite modificar el resultado y que el consumidor recibe el cambio; esa comprobación del host no se hizo durante esta auditoría.

#### H-06 — Registro parcial sin rollback

- **Categoría:** Fuga de recursos.
- **Severidad:** Media.
- **Ubicación:** [infrastructure/plugin/opencode/gomemory.ts:632](/Users/josegomezj/home/rcw/gomemory/infrastructure/plugin/opencode/gomemory.ts:632).
- **Análisis de Vulnerabilidad:** `createV2Setup` inicia la suscripción antes de registrar todos los hooks. Si un registro rechaza, setup termina sin devolver su función de cleanup. La suscripción y los registros anteriores permanecen activos, reteniendo closures y estado del plugin.
- **Evidencia:** Un fallo al registrar `context` produjo rechazo de setup con la suscripción sin abortar y cero registros dispuestos.
- **Código Corregido (Seguro y Defensivo):** Reemplazo íntegro de `createV2Setup`, compatible con el helper de H-05. La suscripción queda esperable y el rollback dispone los registros previos. En fallo de setup no se cierra una sesión de memoria que pudiera pertenecer a otra instancia.

```typescript
const createV2Setup = (exec: Exec) => async (ctx: any) => {
  const directory: string = ctx.location.directory;
  const hooks: any = await GomemoryPlugin({
    $: bunShellShim(exec),
    directory,
    client: clientShim(ctx),
  } as any);
  const registrations: Array<{ dispose: () => Promise<void> }> = [];
  const controller = new AbortController();

  // Un servicio de OpenCode 2.x aloja varias ubicaciones y entrega los eventos
  // y los ganchos de sesión a TODAS las instancias del plugin (reproducido con
  // 2.0.16 y dos proyectos: la instancia de B registraba el turno de A con el
  // cwd de B y le inyectaba su memoria). Cada instancia atiende solo las
  // sesiones de su directorio. La ubicación de la sesión sale del evento si la
  // trae o de ctx.session.get; el resultado se recuerda por sesión. Una sesión
  // que no se puede resolver se ignora: es preferible perder un checkpoint a
  // guardarlo en la memoria de otro proyecto.
  const owners = new Map<string, boolean>();
  const ownsSession = async (sessionID: string | undefined, hinted?: string): Promise<boolean> => {
    if (!sessionID) return false;
    if (hinted) {
      owners.set(sessionID, hinted === directory);
      return hinted === directory;
    }
    const known = owners.get(sessionID);
    if (known !== undefined) return known;
    // Host sin la API: una sola ubicación, como en 1.x.
    if (typeof ctx.session?.get !== "function") return true;
    try {
      const info = await ctx.session.get({ sessionID });
      const mine = info?.location?.directory === directory;
      owners.set(sessionID, mine);
      return mine;
    } catch {
      return false;
    }
  };

  // v2:event
  // Los eventos de ejecución llegan sin location: su dueño se resuelve con
  // ownsSession.
  //
  // OpenCode 2.0.16 no emite session.idle al terminar un turno (verificado con
  // el binario): el fin de turno es session.execution.*. Se traducen todos a
  // session.idle; el checkpoint v1 recuerda el último mensaje revisado, así que
  // un idle extra no duplica nada.
  const v1Events: Record<string, string> = {
    "session.created": "session.created",
    "session.idle": "session.idle",
    "session.execution.succeeded": "session.idle",
    "session.execution.failed": "session.idle",
    "session.execution.interrupted": "session.idle",
    "session.deleted": "session.deleted",
    "session.compaction.ended": "session.compacted",
  };
  const subscription = (async () => {
    try {
      for await (const ev of ctx.event.subscribe({ signal: controller.signal })) {
        if (controller.signal.aborted) break;
        const type = v1Events[ev?.type];
        if (!type) continue;
        const sessionID = ev.data?.sessionID;
        const hinted = ev.data?.location?.directory ?? ev.location?.directory;
        if (!(await ownsSession(sessionID, hinted))) continue;
        await safe(() => hooks.event({ event: { type, properties: { sessionID, info: { id: sessionID } } } }));
        if (type === "session.deleted") owners.delete(sessionID);
      }
    } catch {
      // la suscripción termina al abortar o al descargarse el plugin
    }
  })();

  const cleanup = async (closeSession = true): Promise<void> => {
    controller.abort();
    for (const r of registrations) await safe(() => r.dispose());
    await subscription;
    owners.clear();
    if (closeSession) await safe(() => hooks.dispose());
  };

  try {
  // v2:prompt
  registrations.push(
    await ctx.session.hook("prompt", async (ev: any) =>
      (await ownsSession(ev.sessionID)) &&
      safe(() => hooks["chat.message"]({ sessionID: ev.sessionID }, { parts: [{ type: "text", text: ev.prompt?.text ?? "" }] })),
    ),
  );

  // v2:context
  registrations.push(
    await ctx.session.hook("context", (ev: any) =>
      safe(async () => {
        if (!(await ownsSession(ev.sessionID))) return;
        const out = { system: [] as string[] };
        await hooks["experimental.chat.system.transform"]({ sessionID: ev.sessionID }, out);
        for (const text of out.system) ev.system.push({ type: "text", text });
      }),
    ),
  );

  // v2:compaction
  // Corre sobre la petición de resumen: lo que se añade al system lo tiene en
  // cuenta el compresor, igual que output.context en v1.
  registrations.push(
    await ctx.session.hook("compaction", (ev: any) =>
      safe(async () => {
        if (!(await ownsSession(ev.sessionID))) return;
        const out = { context: [] as string[] };
        await hooks["experimental.session.compacting"]({ sessionID: ev.sessionID }, out);
        for (const text of out.context) ev.system.push({ type: "text", text });
      }),
    ),
  );

  // v2:tool.execute.after
  registrations.push(
    await ctx.tool.hook("execute.after", (ev: any) =>
      safe(async () => {
        if (ev.status !== "completed" || !(await ownsSession(ev.sessionID))) return;
        const original = resultText(ev.result);
        const output = { output: original };
        await hooks["tool.execute.after"](
          { tool: v1ToolName(ev.tool), sessionID: ev.sessionID, callID: ev.id },
          output,
        );
        if (output.output !== original) applyResultText(ev.result, output.output);
      }),
    ),
  );

  return () => cleanup();
  } catch (error) {
    await cleanup(false);
    throw error;
  }
};
```

- **Validación de la propuesta:** La reproducción pasa de suscripción activa y cero disposiciones a suscripción abortada y una disposición. El plugin propuesto, junto con las pruebas corregidas de H-09, pasa 24/24 en una copia temporal. La propuesta todavía requiere verificación en el ciclo de vida del host real.
- **Cierre:** Fallar cada registro por separado, disponer el plugin mientras hay un evento en curso y comprobar que no llegan llamadas después del cierre. Incluir casos donde falla `dispose` y donde la suscripción rechaza.

#### H-07 — Éxito falso de wrap

- **Categoría:** Error lógico.
- **Severidad:** Media.
- **Ubicación:** [adapters/primary/cli/cmd_wrap.go:43](/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_wrap.go:43).
- **Análisis de Vulnerabilidad:** Solo se trata `*exec.ExitError`. Un error de arranque —comando inexistente o sin permisos— mantiene el código cero, oculta el fallo y abre el diálogo de guardado. En automatización puede interpretarse como ejecución satisfactoria.
- **Evidencia:** El binario real terminó 0 al ejecutar `wrap -s=false` con un comando inexistente y stdin cerrado.
- **Código Corregido (Seguro y Defensivo):** Sustituir la ejecución y clasificación del error por este bloque antes del diálogo.

```go
runErr := cmd.Run()
exitCode := 0
if runErr != nil {
 if exitErr, ok := runErr.(*exec.ExitError); ok {
  exitCode = exitErr.ExitCode()
 } else {
  fmt.Fprintln(os.Stderr,
   "WRAP_START_FAILED: no se pudo iniciar el comando; comprueba la ruta y los permisos.")
  os.Exit(1)
 }
}
```

- **Validación de la propuesta:** El binario de la copia temporal devuelve 1; las regresiones CLI de update/wrap seleccionadas pasan.
- **Cierre:** Probar comando ausente, permisos de ejecución denegados, éxito y un hijo que termina con un código no cero. El fallo de arranque no debe preguntar por guardado.

#### H-08 — Integridad inconsistente entre instaladores

- **Categoría:** Inseguridad / integridad.
- **Severidad:** Media.
- **Ubicación:** [scripts/install.sh:135](/Users/josegomezj/home/rcw/gomemory/scripts/install.sh:135) y [scripts/install.ps1:101](/Users/josegomezj/home/rcw/gomemory/scripts/install.ps1:101).
- **Análisis de Vulnerabilidad:** Ambos scripts extraen e instalan el archivo descargado sin consultar `checksums.txt`. Go y TypeScript sí lo verifican. Un archivo alterado que siga siendo un archivo comprimido válido puede instalarse silenciosamente. No se demostró evasión de HTTPS ni se presenta el checksum del mismo origen como firma independiente del proveedor.
- **Evidencia:** Un curl simulado entregó un tar con ejecutable sintético; install.sh lo instaló sin solicitar checksums. La vía PowerShell se confirmó por fuente, sin ejecución.
- **Código Corregido (Seguro y Defensivo):** En Bash, sustituir la descarga/extracción dentro de `main`, después de crear `tmp` y su trap. Usar un tag fijo para que el archivo y su checksum pertenezcan al mismo release.

```bash
local resolved_version="$VERSION"
  if [ "$resolved_version" = latest ]; then
    local latest_url
    latest_url="$(curl -fsSL --connect-timeout 10 --max-time 30 \
      -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")" \
      || die "No se pudo resolver el release"
    case "$latest_url" in
      "https://github.com/${REPO}/releases/tag/"*) ;;
      *) die "Destino de release inesperado" ;;
    esac
    resolved_version="${latest_url##*/}"
  fi
  [[ "$resolved_version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]] \
    || die "Versión estable inválida"
  local base="https://github.com/${REPO}/releases/download/${resolved_version}"
  curl -fsSL --connect-timeout 10 --max-time 120 \
    "$base/$asset" -o "$tmp/$asset" || die "Descarga fallida"
  curl -fsSL --connect-timeout 10 --max-time 30 \
    "$base/checksums.txt" -o "$tmp/checksums.txt" || die "Falta checksums.txt"

  local expected actual
  expected="$(awk -v name="$asset" '
    ($2 == name || $2 == "*" name) && length($1) == 64 &&
    $1 ~ /^[[:xdigit:]]+$/ { print tolower($1) }
  ' "$tmp/checksums.txt")"
  [[ "$expected" =~ ^[a-f0-9]{64}$ ]] || die "Checksum ausente o ambiguo"
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
  else
    die "Se requiere sha256sum o shasum"
  fi
  [ "$actual" = "$expected" ] || die "Checksum incorrecto"
  tar -xzf "$tmp/$asset" -C "$tmp" -- "$BIN_NAME"
  [ -f "$tmp/$BIN_NAME" ] && [ ! -L "$tmp/$BIN_NAME" ] \
    && [ -s "$tmp/$BIN_NAME" ] || die "Ejecutable inválido"
```

En PowerShell, sustituir el bloque de descarga/extracción dentro del `try` existente:

```powershell
if ($Version -eq "latest") {
  $releaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = [string]$releaseInfo.tag_name
}
if ($Version -notmatch '^v?\d+\.\d+\.\d+$') {
  throw "Versión estable inválida"
}
$releaseBase = "https://github.com/$Repo/releases/download/$Version"
$zip = Join-Path $tmp $asset
$checksumFile = Join-Path $tmp "checksums.txt"
Invoke-WebRequest -Uri "$releaseBase/$asset" -OutFile $zip -UseBasicParsing
Invoke-WebRequest -Uri "$releaseBase/checksums.txt" -OutFile $checksumFile -UseBasicParsing
$checksumPattern = '^([a-fA-F0-9]{64})\s+\*?' + [regex]::Escape($asset) + '$'
$checksumLines = @(Get-Content $checksumFile | Where-Object { $_ -match $checksumPattern })
if ($checksumLines.Count -ne 1) { throw "Checksum ausente o ambiguo" }
$expectedHash = ($checksumLines[0] -split '\s+')[0]
$actualHash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash
if ($actualHash -ine $expectedHash) { throw "Checksum incorrecto" }
Expand-Archive -Path $zip -DestinationPath $tmp -Force
$executable = Get-Item -LiteralPath (Join-Path $tmp $BinName)
if ($executable.PSIsContainer -or $executable.Length -eq 0) {
  throw "Ejecutable inválido"
}
```

- **Validación de la propuesta:** Bash pasa el análisis sintáctico. En el fixture de checksum incorrecto solicita checksums, termina 1 y deja el destino sin escribir. La propuesta PowerShell requiere pruebas Windows.
- **Cierre:** Comprobar hash válido, inválido, ausente, duplicado y fallo de red, conservando un binario previo. Estas propuestas admiten releases estables; documentar ese límite o extender explícitamente la validación si se decide soportar prereleases.

#### H-09 — Contrato de pruebas desactualizado

- **Categoría:** Error lógico / calidad de pruebas.
- **Severidad:** Baja.
- **Ubicación:** [infrastructure/plugin/opencode/gomemory.test.mjs:145](/Users/josegomezj/home/rcw/gomemory/infrastructure/plugin/opencode/gomemory.test.mjs:145) y [infrastructure/plugin/opencode/gomemory.test.mjs:288](/Users/josegomezj/home/rcw/gomemory/infrastructure/plugin/opencode/gomemory.test.mjs:288).
- **Análisis de Vulnerabilidad:** Los asserts comparan toda la llamada con `session start`, aunque el contrato actual añade `--conversation=<id>`. Dos pruebas fallan por esta diferencia. El mock de fallos que compara `session start` por igualdad tampoco dispara para la llamada actual, debilitando el caso de manejo de errores.
- **Evidencia:** La suite original da 22/24. Una copia que cambia únicamente los filtros al prefijo semántico de argumentos da 24/24.
- **Código Corregido (Seguro y Defensivo):** Filtrar por los primeros argumentos y verificar el ID de conversación; liberar recursos aunque falle un assert.

```javascript
const sessionStarts = f => f.calls.filter(
  c => c.args[0] === 'session' && c.args[1] === 'start',
);

test('v2: session.created starts a session only for this directory', async () => {
  const f = await v2Fixture();
  try {
    await f.emit('session.created', { sessionID: 's1' }, '/otro');
    assert.equal(sessionStarts(f).length, 0);
    await f.emit('session.created');
    assert.equal(sessionStarts(f).length, 1);
    assert.deepEqual(sessionStarts(f)[0].args, ['session', 'start', '--conversation=s1']);
    assert.equal(sessionStarts(f)[0].cwd, '/proj');
  } finally {
    await f.cleanup();
  }
});

test('v2: session.created uses the session location, not the envelope', async () => {
  const f = await v2Fixture({ owners: {} });
  try {
    await f.emit('session.created', { sessionID: 'sB', location: { directory: '/projB' } }, null);
    await f.emit('session.created', { sessionID: 'sP', location: { directory: '/proj' } }, '/home');
    assert.equal(sessionStarts(f).length, 1);
    assert.deepEqual(sessionStarts(f)[0].args, ['session', 'start', '--conversation=sP']);
    assert.ok((await f.system('context', 'sP')).length > 0);
    assert.deepEqual(await f.system('context', 'sB'), []);
  } finally {
    await f.cleanup();
  }
});
```

En el fake `exec` de `v2Fixture`, usar este bloque para los fallos configurados:

```javascript
const command = args.join(' ');
if (out.fail?.some(prefix => command === prefix || command.startsWith(prefix + ' '))) {
 throw new Error('boom');
}
```

- **Validación de la propuesta:** Los filtros corregidos pasan 24/24. También pasa 24/24 la propuesta integral con los asserts adicionales del ID, el mock ampliado y las correcciones de H-05/H-06, aplicada únicamente en temporales. Incorporar esos casos a la regresión permanente.
- **Cierre:** Ejecutar la suite JS del plugin en CI junto con Go y el instalador. Cambios de argumentos del protocolo deben actualizar producción y fixtures juntos.

### 3. Recomendaciones de Resiliencia

1. **Cerrar primero H-01 y H-02.** Aplicar la misma política de privacidad a texto libre que se persiste o imprime, y probar todos los bordes con marcadores sintéticos. Para diagnósticos técnicos, registrar códigos y metadatos permitidos; evitar volcar prompts, argumentos o excepciones completas. OWASP recomienda excluir secretos y datos sensibles de los logs. [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html).
2. **Incorporar las reproducciones de H-03 a las pruebas.** Una suite verde con `-race` solo cubre las intercalaciones realmente ejecutadas; la documentación de Go explica este límite. Mantener pruebas de llamadas simultáneas sobre instancias compartidas. [Go Data Race Detector](https://go.dev/doc/articles/race_detector).
3. **Cerrar recursos durante setup parcial y shutdown.** Esperar la tarea de suscripción, disponer registros, remover listeners y acotar las esperas de procesos externos. Para listeners Node, aprovechar señales de aborto y disposiciones explícitas según las APIs utilizadas. [Node.js Events](https://nodejs.org/api/events.html).
4. **Acotar trabajo antes de asignar o transformar.** Revisar lectura de stdin de hooks, tamaño de respuestas de descarga, líneas NDJSON y descompresión de originales. Definir límites por contrato y probar exceso de tamaño, datos incompletos, timeout y cancelación. La auditoría no demuestra una fuga permanente en estos caminos; son oportunidades de defensa en profundidad.
5. **Uniformar integridad y publicación de artefactos.** Fijar release, verificar checksum antes de extraer y preparar archivos antes de publicar el destino. Añadir pruebas Windows para descarga, cancelación, restauración y permisos.
6. **Normalizar errores en bordes MCP.** Separar errores de validación y fallos internos, usar códigos estables y un identificador de correlación cuando ayude al diagnóstico. Mantener los detalles internos depurados fuera de las respuestas al agente, sin romper los contratos JSON/NDJSON existentes.
7. **Hacer explícita la matriz de soporte.** Python 3.12 para los scripts que usan su sintaxis, Node mínimo declarado por el instalador y pruebas reales por plataforma. La ausencia de `pwsh` y de validación del host OpenCode limita las comprobaciones efectuadas.
8. **Repetir análisis de dependencias en CI.** Mantener govulncheck y añadir npm audit con el alcance documentado; cero avisos en esta consulta no valida privacidad, recursos ni lógica propia.

**Estado final:** Auditoría completada. Producción sin cambios. Las correcciones y pruebas propuestas quedan listas para revisión; su integración y la limpieza controlada de datos históricos son trabajo posterior.


### 4. Estado de cierre (feature 037)

Correcciones implementadas en `specs/037-close-audit-gaps` sobre esta misma revisión base, todavía sin commit. El detalle por hallazgo (causa, comando, resultado esperado y observado, entorno y evidencia depurada) está en [specs/037-close-audit-gaps/evidence/closure.md](../../specs/037-close-audit-gaps/evidence/closure.md).

| Hallazgo | Estado | Evidencia pendiente |
| --- | --- | --- |
| H-01 — Redacción incompleta | Cerrado | — |
| H-02 — Bloques privados | Cerrado | — |
| H-03 — Estado compartido | Cerrado | — |
| H-04 — Actualización Unix | Cerrado | — |
| H-05 — Compresión en OpenCode v2 | Corregido, pendiente de verificación | Consumidor real en OpenCode 2.x |
| H-06 — Rollback y cierre del plugin | Corregido, pendiente de verificación | Cierre real en OpenCode 2.x |
| H-07 — Éxito falso de wrap | Cerrado | — |
| H-08 — Integridad de instaladores | Corregido; Unix cerrado | `install.ps1` en Windows real |
| H-09 — Pruebas desactualizadas | Cerrado | Primera ejecución remota del workflow `test` |

Hallazgos adicionales durante el cierre, ya corregidos: `mem compare -m` repetía el razonamiento crudo; la verificación de residuos de `mem sanitize` entraba en bucle ante remanentes de bloques anidados; y la cancelación de subprocesos del plugin no alcanzaba a sus descendientes.
