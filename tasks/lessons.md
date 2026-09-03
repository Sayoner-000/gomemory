# Lecciones

Patrones aprendidos de correcciones reales. Cada entrada existe para evitar que el mismo error se repita.

---

## L001 — Un comando en documentación operativa se verifica contra el código, no por analogía

**Fecha**: 2026-09-02 · **Contexto**: redacción de `specs/027-octopus-aar/quickstart.md`

**Qué pasó**: escribí pasos de verificación con `mem mcp --list-tools` y `mem doctor --db-path`, y con la base de datos en `.memory/mem.db`. Ninguna de las tres cosas existe: `mem mcp` habla JSON-RPC por stdio sin bandera de listado, `doctor` no tiene `--db-path`, y la base vive en el store global (`~/.local/share/gomemory/projects/<clave>/mem.db`); `.memory/mem.db` es el legado que `globalstore.go` migra.

**Causa raíz**: redacté por analogía con otros proyectos en vez de leer las banderas realmente registradas en el adaptador del CLI.

**Por qué importa**: una guía de validación que invoca comandos inexistentes es la misma clase de fallo que un test con fixture que no refleja el upstream — da sensación de verificación sin verificar nada. Es la regla 2 de las reglas de campo aplicada a documentación, no solo a tests.

**Regla preventiva**: antes de escribir un comando en un quickstart, un README o cualquier documento operativo, comprobar que existe: `grep` de la bandera en el adaptador del CLI, o ejecutarlo. Si no se puede ejecutar, no se escribe.

**Aplicada en**: `specs/027-octopus-aar/quickstart.md` §2 y §9 · memoria gomemory id=10

---

## L002 — `flag.Parse` se detiene en el primer argumento posicional

**Fecha**: 2026-09-02 · **Contexto**: `mem octopus route "<objetivo>" --class investigation --read-only`

**Qué pasó**: el paquete `flag` deja de parsear al encontrar el primer argumento que no empieza por `-`. Con el objetivo en primera posición, TODAS las banderas posteriores quedaban dentro de `fs.Args()` y conservaban su valor por defecto. El comando respondía `INLINE` siempre, con cifras plausibles.

**Por qué ninguna prueba lo vio**: la política recibía una entrada perfectamente bien formada — solo que equivocada. Ninguna prueba de dominio puede detectar eso; el defecto vive en la traducción de argumentos, no en la decisión.

**Regla preventiva**: cuando un comando mezcla posicionales y banderas, extraer los posicionales ANTES de `fs.Parse`, y cubrirlo con una prueba que pase las banderas DESPUÉS del posicional. Y ejecutar el binario con la invocación real que usará la persona, no solo la suite.

---

## L003 — Una prueba de ausencia necesita una aserción de control

**Fecha**: 2026-09-02 · **Contexto**: aislamiento de contexto y huella cero del módulo Octopus

**Qué pasó**: dos pruebas distintas estuvieron a punto de pasar sin medir nada. "El paquete no contiene memorias ajenas" pasaría con un paquete vacío. "Cero filas en la tabla con el módulo apagado" pasaría con la telemetría rota.

**Regla preventiva**: toda prueba que afirme una AUSENCIA lleva al lado una aserción que demuestre que el mecanismo funciona cuando debe: que lo relevante SÍ entró, que con el módulo encendido SÍ se escribe. Sin ese control, la prueba solo demuestra que no pasó nada — incluido lo que debía pasar.

---

## L004 — `gofmt -w` sobre un directorio reformatea código ajeno al cambio

**Fecha**: 2026-09-02 · **Contexto**: implementación de la feature 027

**Qué pasó**: ejecutar `gofmt -w adapters/primary/cli/` y `application/usecases/` reformateó 7 archivos que no tenían nada que ver con la funcionalidad, mezclando ruido con el cambio real en el diff.

**Regla preventiva**: `gofmt -w` se ejecuta sobre los ARCHIVOS tocados, nunca sobre el directorio. Antes de dar por terminado, comparar `gofmt -l .` contra la línea base tomada al empezar: cualquier diferencia nueva es atribuible al cambio y hay que justificarla o revertirla.

---

## L005 — Un fixture que inyecta a mano lo que producción no inyecta valida tus suposiciones, no el sistema

**Fecha**: 2026-09-02 · **Contexto**: revisión adversarial de la funcionalidad 027 (Octopus AAR), 31 hallazgos con la suite entera en verde

**Qué pasó**: la funcionalidad quedó en buena medida inerte en producción mientras 107 tareas figuraban completas y 13 paquetes de pruebas pasaban. Cinco caminos documentados no tenían ruta de ejecución alcanzable.

**Causa raíz, idéntica en los cinco casos**: cada prueba inyectaba a mano el dato que en producción nadie inyecta.

| La prueba pasaba | Porque inyectaba | Y en producción |
|---|---|---|
| Delegación | `ContextNeed.EstimatedTokens: 2200` | Se mide solo la frase de objetivo, ~25 tokens, por debajo del mínimo delegable |
| Grupos paralelos | `MaxParallel: 3` explícito | El esquema lo marca opcional y `Normalize()` lo fija en 1 |
| Paquete de contexto | Un `memRepo` real | El único llamador pasa `nil` y la función corta en la primera línea |
| Auto-aprobación | Se comprobaba `SettingsData` en memoria | El archivo escrito nunca recibe los nombres |
| Política de fallos | Se llamaba `HandleFailure` directo | Ningún adaptador la invoca |

**Regla preventiva** — antes de dar una funcionalidad por terminada, para CADA camino que la documentación prometa:

1. Ejecutarlo por la superficie real que usará quien lo consuma (tool MCP, comando sin banderas, archivo de configuración), no por el caso de uso desde una prueba.
2. Preguntar de cada dato que la prueba construye: *¿quién lo produce en producción?* Si la respuesta es "el fixture", hay un camino sin cubrir.
3. `grep` de cada símbolo nuevo excluyendo `_test.go`. Si no aparece ningún llamador de producción, es código muerto por muy verde que esté su prueba.

**Corolario sobre las tablas de casos**: una tabla donde cada fila activa una sola condición sobre una entrada por lo demás sana NO prueba el orden de evaluación. Para fijar un orden hace falta un caso por par adyacente con AMBAS condiciones ciertas, comprobando cuál gana.

---

## Plan de cierre pendiente para la funcionalidad 027

Orden propuesto, de mayor a menor rendimiento por esfuerzo. Nada de esto está hecho.

1. **Estimar el contexto en el borde, una sola vez.** Mover la compensación que hoy solo tiene el CLI (`leerAlcance`) a `RouteTaskUseCase`/`RoutePlanUseCase`, de modo que MCP y CLI compartan el mismo camino. Desbloquea la delegación por la superficie principal.
2. **Cablear `PackContractUseCase` con sus dependencias reales** en el composition root y llamarlo desde las rutas delegadas. Devuelve la vida a la redacción de secretos y al filtro de aislamiento.
3. **Devolver la recomendación de fallo** en la respuesta de `octopus_report`.
4. **Derivar la auto-aprobación del estado del módulo** en el punto donde se escribe, no en una variable de paquete.
5. **Descontar de la reserva** en `Budget.Gastar` cuando esté autorizada, y dejar de imprimir "intacta" sin comprobarlo.
6. **Acotar los agregados a filas reportadas** y separar la tasa de éxito por ruta.
7. **Decidir sobre `Risk`, `CriticalPath` y `DuplicateWork`**: implementarlos o retirarlos de los esquemas y de la especificación. Hoy la interfaz promete lo que la política no lee.
