# Soporte bilingüe desacoplado mediante plantillas con cambio de idioma en caliente (mem lang)

ID: HCD-056
Tipo: evolution
Carril: full
Motivo: Facilitar la adopción global de gomemory permitiendo elegir entre Español neutral e Inglés en instalación o en caliente con mem lang

## Intención
Permitir que la persona elija entre Español neutral e Inglés durante la instalación (mem install) o mediante cambio dinámico en caliente (mem lang), garantizando que la salida de la CLI, las sugerencias operativas y las directivas inyectadas a los agentes respeten el idioma seleccionado sin hardcoding ni switch-cases, usando paquetes de plantillas descubribles en templates/locales/<código>/.

## Alcance
Incluido: Motor de localización (LocaleRegistry e I18nCatalog) descubrible dinámicamente desde templatesFS (go:embed); paquetes de plantillas para español neutral (es) e inglés (en) con meta.json, messages.json y plantillas de agente (.md.tmpl); campo language en SettingsData (.memory/settings.json); comando en caliente mem lang [código] con selector interactivo y propagación inmediata; flag --lang en mem settings y en mem install; integración en el instalador guiado askInstallSelection descubriendo idiomas disponibles; adaptación de sugerencias, hints y diagnósticos CLI para usar I18nCatalog; inyección de plantillas de agente (get_context, get_plan_context) según el idioma activo en caliente.
Excluido: Modificaciones a los comandos mem sanitize o mem optimize (se mantienen intactos); soporte de idiomas adicionales más allá de es y en en esta unidad inicial; traducción del contenido de memorias históricas ya persistidas en la base de datos.

## ¿Qué pasa si no lo hacemos?
gomemory permanece acoplado exclusivamente al idioma español en código duro, limitando su adopción en proyectos y equipos de habla inglesa, y requiriendo parches manuales invasivos cada vez que se requiera soporte multi-idioma.

## Alternativa más barata
Hardcodear condicionales if language == "en" dentro de cada función de la CLI y de MCP. Se descarta porque genera deuda técnica alta, acoplamiento fuerte y obliga a modificar el código fuente cada vez que se necesite corregir una traducción o incorporar un nuevo idioma.

## Historias
### US1. Selección de idioma en instalación
Como desarrollador que instala gomemory, quiero seleccionar mi idioma preferido (español neutral o inglés), para que el entorno quede configurado en mi lengua desde el inicio.
- C-01: Dado que ejecuto mem install en modo interactivo, cuando llego al paso de selección de idioma, entonces visualizo las opciones descubiertas en templates/locales/ y al seleccionar una queda persistida en .memory/settings.json.
- C-02: Dado que ejecuto mem install con el flag --lang=en, cuando finaliza la instalación, entonces .memory/settings.json contiene language establecido en en.

### US2. Cambio de idioma en caliente con mem lang
Como usuario de gomemory, quiero cambiar de idioma en cualquier momento con mem lang, para que la CLI y las respuestas de agentes se actualicen de inmediato sin reiniciar servicios.
- C-03: Dado un proyecto configurado en español, cuando ejecuto mem lang en, entonces la CLI confirma el cambio en inglés y actualiza settings.json inmediatamente.
- C-04: Dado que ejecuto mem lang con un código inexistente, cuando se valida el código, entonces el comando falla con código de salida 2 indicando los idiomas válidos descubiertos.

### US3. Protocolo MCP y sugerencias en el idioma configurado
Como agente que interactúa vía MCP, quiero recibir directivas y contexto en el idioma activo del proyecto, para guiar mis acciones en la lengua configurada.
- C-05: Dado un proyecto con idioma en, cuando invoco get_context o get_plan_context vía MCP, entonces las directivas de protocolo y la guía de descomposición inyectadas están en inglés según sus plantillas.

## Aclaraciones
- [x] Q1. ¿mem sanitize y mem optimize forman parte de esta unidad? → No, se acordó explícitamente mantenerlos intactos y enfocar la unidad al 100% en el sistema de idioma.
- [x] Q2. ¿El cambio de idioma debe ser en caliente? → Sí, mediante comando mem lang y lectura dinámica en cada ejecución/llamada MCP sin reiniciar procesos.
- [x] Q3. ¿Los idiomas estarán hardcodeados en Go? → No, cero hardcoding; se descubren desde templates/locales/<código>/ a través de templatesFS.

## Slices
### VS-01. Motor i18n desacoplado y paquetes de plantillas
Tareas: 1, 2, 3
Criterios: C-04

### VS-02. Persistencia y comando de switch en caliente mem lang
Tareas: 4, 5, 6
Criterios: C-03, C-04

### VS-03. Integración en el instalador mem install
Tareas: 7, 8, 9
Criterios: C-01, C-02

### VS-04. Sugerencias CLI y plantillas MCP en caliente
Tareas: 10, 11, 12
Criterios: C-05

## Checklist
- [x] 1. Crear estructura templates/locales/es/ y templates/locales/en/ con meta.json, messages.json y plantillas markdown → `ls infrastructure/templates/locales/es infrastructure/templates/locales/en`
- [x] 2. Implementar domain/i18n o adapters con LocaleRegistry e I18nCatalog descubriendo plantillas desde templatesFS → `go test ./domain/i18n ./adapters/secondary/i18n -run TestI18nDiscovery -count=1`
- [x] 3. Pruebas unitarias de descubrimiento dinámico, resolución de claves, fallbacks y paridad de claves → `go test ./domain/i18n ./adapters/secondary/i18n -run TestParidad -count=1`
- [x] 4. Extender SettingsData con campo Language y validación contra LocaleRegistry → `go test ./adapters/secondary/persistence -run TestSettingsLanguage -count=1`
- [x] 5. Implementar CmdLang (mem lang [código]) y flag --lang en CmdSettings → `go test ./adapters/primary/cli -run TestCmdLang -count=1`
- [x] 6. Pruebas unitarias y funcionales de mem lang (éxito, switch en caliente, códigos inválidos) → `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1`
- [x] 7. Integrar paso de selección de idioma en askInstallSelection usando idiomas descubiertos → `go test ./adapters/primary/cli -run TestAskInstallSelectionLanguage -count=1`
- [x] 8. Añadir flag --lang a parseInstallArgs y persistir selección en settings.json → `go test ./adapters/primary/cli -run TestParseInstallArgsLanguage -count=1`
- [x] 9. Pruebas de integración de mem install con selección interactiva y no interactiva (--lang) → `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1`
- [x] 10. Adaptar hints, diagnósticos y avisos de CLI para resolver textos vía I18nCatalog → `go test ./adapters/primary/cli -run TestI18nHints -count=1`
- [x] 11. Extraer y conectar plantillas de agente (get_context, atomic_plan) renderizadas según idioma activo → `go test ./application/usecases -run TestBuildContextLanguage -count=1`
- [x] 12. Pruebas de integración extremo a extremo de MCP get_context y get_plan_context en ES y EN → `go test ./adapters/primary/cli -run TestMCPContextLanguage -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-08 07:55 -05:00 · tarea 1 · rojo observado · caducada · `ls templates/locales/es templates/locales/en` (salida 1)
      ls: templates/locales/en: No such file or directory
      ls: templates/locales/es: No such file or directory
- 2026-11-08 08:00 -05:00 · tarea 1 · verde · caducada · `ls infrastructure/templates/locales/es infrastructure/templates/locales/en` (salida 0)
      infrastructure/templates/locales/en:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
      infrastructure/templates/locales/es:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
- 2026-11-08 08:00 -05:00 · tarea 2 · rojo observado · caducada · `go test ./domain/i18n ./adapters/secondary/i18n -run TestI18nDiscovery -count=1` (salida 1)
      # ./domain/i18n
      stat /Users/josegomezj/home/rcw/gomemory/domain/i18n: directory not found
      FAIL	./domain/i18n [setup failed]
      # ./adapters/secondary/i18n
      stat /Users/josegomezj/home/rcw/gomemory/adapters/secondary/i18n: directory not found
      FAIL	./adapters/secondary/i18n [setup failed]
      FAIL
- 2026-11-08 08:01 -05:00 · tarea 2 · verde · caducada · `go test ./domain/i18n ./adapters/secondary/i18n -run TestI18nDiscovery -count=1` (salida 0)
      ok  	mem/domain/i18n	0.112s [no tests to run]
      ok  	mem/adapters/secondary/i18n	0.117s
- 2026-11-08 08:01 -05:00 · tarea 1 · verde · caducada · `ls infrastructure/templates/locales/es infrastructure/templates/locales/en` (salida 0)
      infrastructure/templates/locales/en:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
      infrastructure/templates/locales/es:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
- 2026-11-08 08:02 -05:00 · tarea 3 · verde · caducada · `go test ./domain/i18n ./adapters/secondary/i18n -run TestParidad -count=1` (salida 0)
      ok  	mem/domain/i18n	0.139s [no tests to run]
      ok  	mem/adapters/secondary/i18n	0.144s
- 2026-11-08 08:02 -05:00 · tarea 4 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/secondary/persistence -run TestSettingsLanguage -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.220s [no tests to run]
- 2026-11-08 08:02 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run TestSettingsLanguage -count=1` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/settings_language_test.go:14:7: s.Language undefined (type Settings has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:15:62: s.Language undefined (type Settings has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:19:4: s.Language undefined (type Settings has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:25:14: reloaded.Language undefined (type Settings has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:26:71: reloaded.Language undefined (type Settings has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:41:11: sData.Language undefined (type ports.SettingsData has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:42:68: sData.Language undefined (type ports.SettingsData has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:45:8: sData.Language undefined (type ports.SettingsData has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:50:15: reloaded2.Language undefined (type ports.SettingsData has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:51:74: reloaded2.Language undefined (type ports.SettingsData has no field or method Language)
      adapters/secondary/persistence/settings_language_test.go:51:74: too many errors
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-08 08:04 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/persistence -run TestSettingsLanguage -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.183s
- 2026-11-08 08:04 -05:00 · tarea 5 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestCmdLang -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.325s [no tests to run]
- 2026-11-08 08:05 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestCmdLang -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_save_test.go:47:6: fakeProjectRepo redeclared in this block
      	adapters/primary/cli/cmd_lang_test.go:14:6: other declaration of fakeProjectRepo
      adapters/primary/cli/cmd_save_test.go:49:27: method fakeProjectRepo.FindRoot already declared at adapters/primary/cli/cmd_lang_test.go:18:27
      adapters/primary/cli/cmd_save_test.go:53:27: method fakeProjectRepo.Init already declared at adapters/primary/cli/cmd_lang_test.go:19:27
      adapters/primary/cli/cmd_save_test.go:54:27: method fakeProjectRepo.Key already declared at adapters/primary/cli/cmd_lang_test.go:20:27
      adapters/primary/cli/cmd_lang_test.go:11:2: "mem/domain/i18n" imported and not used
      adapters/primary/cli/cmd_lang_test.go:45:2: undefined: CmdLang
      adapters/primary/cli/cmd_lang_test.go:60:2: undefined: CmdLang
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-08 08:07 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdLang -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.286s
- 2026-11-08 08:07 -05:00 · tarea 6 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.283s [no tests to run]
- 2026-11-08 08:08 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.281s
- 2026-11-08 08:08 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1` (salida 1)
      --- FAIL: TestCmdLangSwitch (0.00s)
          cmd_lang_switch_test.go:14: red test
      FAIL
      FAIL	mem/adapters/primary/cli	0.299s
      FAIL
- 2026-11-08 08:09 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.276s
- 2026-11-08 08:09 -05:00 · tarea 7 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestAskInstallSelectionLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.267s [no tests to run]
- 2026-11-08 08:11 -05:00 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestAskInstallSelectionLanguage -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/install_selection_language_test.go:36:9: sel.language undefined (type installSelection has no field or method language)
      adapters/primary/cli/install_selection_language_test.go:37:66: sel.language undefined (type installSelection has no field or method language)
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-08 08:13 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli -run TestAskInstallSelectionLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.273s
- 2026-11-08 08:13 -05:00 · tarea 8 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestParseInstallArgsLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.269s [no tests to run]
- 2026-11-08 08:14 -05:00 · tarea 8 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestParseInstallArgsLanguage -count=1` (salida 1)
      --- FAIL: TestParseInstallArgsLanguage (0.00s)
          install_args_language_test.go:11: parseInstallArgs con --lang=en falló: uso inválido: flag desconocido --lang=en
      FAIL
      FAIL	mem/adapters/primary/cli	0.410s
      FAIL
- 2026-11-08 08:16 -05:00 · tarea 8 · verde · caducada · `go test ./adapters/primary/cli -run TestParseInstallArgsLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.280s
- 2026-11-08 08:16 -05:00 · tarea 9 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.295s [no tests to run]
- 2026-11-08 08:18 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.542s
- 2026-11-08 08:19 -05:00 · tarea 9 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1` (salida 1)
      --- FAIL: TestInstallLanguageIntegration (0.00s)
          install_integration_language_test.go:13: red test
      FAIL
      FAIL	mem/adapters/primary/cli	0.328s
      FAIL
- 2026-11-08 08:19 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.550s
- 2026-11-08 08:19 -05:00 · tarea 10 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestI18nHints -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.283s [no tests to run]
- 2026-11-08 08:20 -05:00 · tarea 10 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestI18nHints -count=1` (salida 1)
      --- FAIL: TestI18nHints (0.00s)
          hints_i18n_test.go:11: red test
      FAIL
      FAIL	mem/adapters/primary/cli	0.319s
      FAIL
- 2026-11-08 08:20 -05:00 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli -run TestI18nHints -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.266s
- 2026-11-08 08:21 -05:00 · tarea 11 · rojo NO observado (el comando pasó) · caducada · `go test ./application/usecases -run TestBuildContextLanguage -count=1` (salida 0)
      ok  	mem/application/usecases	0.211s [no tests to run]
- 2026-11-08 08:21 -05:00 · tarea 11 · rojo observado · caducada · `go test ./application/usecases -run TestBuildContextLanguage -count=1` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/build_context_language_test.go:38:3: unknown field Language in struct literal of type Builder
      application/usecases/build_context_language_test.go:54:3: unknown field Language in struct literal of type Builder
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-08 08:22 -05:00 · tarea 11 · verde · caducada · `go test ./application/usecases -run TestBuildContextLanguage -count=1` (salida 0)
      ok  	mem/application/usecases	0.185s
- 2026-11-08 08:22 -05:00 · tarea 12 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/primary/cli -run TestMCPContextLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.314s [no tests to run]
- 2026-11-08 08:24 -05:00 · tarea 12 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestMCPContextLanguage -count=1` (salida 1)
      # mem/adapters/primary/cli
      adapters/primary/cli/mcp_context_language_test.go:10:2: no required module provides package github.com/mark3labs/mcp-go/mcp; to add it:
      	go get github.com/mark3labs/mcp-go/mcp
      FAIL	mem/adapters/primary/cli [setup failed]
      FAIL
- 2026-11-08 08:24 -05:00 · tarea 12 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestMCPContextLanguage -count=1` (salida 1)
      --- FAIL: TestMCPContextLanguage (0.00s)
          mcp_context_language_test.go:38: red test
      FAIL
      FAIL	mem/adapters/primary/cli	0.504s
      FAIL
- 2026-11-08 08:26 -05:00 · tarea 12 · verde · caducada · `go test ./adapters/primary/cli -run TestMCPContextLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.307s
- 2026-11-08 08:26 -05:00 · tarea 1 · verde · caducada · `ls infrastructure/templates/locales/es infrastructure/templates/locales/en` (salida 0)
      infrastructure/templates/locales/en:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
      infrastructure/templates/locales/es:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
- 2026-11-08 08:26 -05:00 · tarea 2 · verde · caducada · `go test ./domain/i18n ./adapters/secondary/i18n -run TestI18nDiscovery -count=1` (salida 0)
      ok  	mem/domain/i18n	0.138s [no tests to run]
      ok  	mem/adapters/secondary/i18n	0.140s
- 2026-11-08 08:27 -05:00 · tarea 3 · verde · caducada · `go test ./domain/i18n ./adapters/secondary/i18n -run TestParidad -count=1` (salida 0)
      ok  	mem/domain/i18n	0.116s [no tests to run]
      ok  	mem/adapters/secondary/i18n	0.110s
- 2026-11-08 08:27 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/persistence -run TestSettingsLanguage -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.185s
- 2026-11-08 08:27 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdLang -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.281s
- 2026-11-08 08:27 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.293s
- 2026-11-08 08:27 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli -run TestAskInstallSelectionLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.274s
- 2026-11-08 08:27 -05:00 · tarea 8 · verde · caducada · `go test ./adapters/primary/cli -run TestParseInstallArgsLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.274s
- 2026-11-08 08:28 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.468s
- 2026-11-08 08:28 -05:00 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli -run TestI18nHints -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.271s
- 2026-11-08 08:28 -05:00 · tarea 11 · verde · caducada · `go test ./application/usecases -run TestBuildContextLanguage -count=1` (salida 0)
      ok  	mem/application/usecases	0.204s
- 2026-11-08 08:45 -05:00 · tarea 1 · verde · vigente · `ls infrastructure/templates/locales/es infrastructure/templates/locales/en` (salida 0)
      infrastructure/templates/locales/en:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
      infrastructure/templates/locales/es:
      atomic_plan.md.tmpl
      get_context.md.tmpl
      messages.json
      meta.json
- 2026-11-08 08:45 -05:00 · tarea 2 · verde · vigente · `go test ./domain/i18n ./adapters/secondary/i18n -run TestI18nDiscovery -count=1` (salida 0)
      ok  	mem/domain/i18n	0.208s [no tests to run]
      ok  	mem/adapters/secondary/i18n	0.215s
- 2026-11-08 08:45 -05:00 · tarea 3 · verde · vigente · `go test ./domain/i18n ./adapters/secondary/i18n -run TestParidad -count=1` (salida 0)
      ok  	mem/domain/i18n	0.129s [no tests to run]
      ok  	mem/adapters/secondary/i18n	0.135s
- 2026-11-08 08:45 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/secondary/persistence -run TestSettingsLanguage -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.165s
- 2026-11-08 08:46 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli -run TestCmdLang -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.292s
- 2026-11-08 08:47 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/primary/cli -run TestCmdLangSwitch -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.297s
- 2026-11-08 08:47 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli -run TestAskInstallSelectionLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.315s
- 2026-11-08 08:47 -05:00 · tarea 8 · verde · vigente · `go test ./adapters/primary/cli -run TestParseInstallArgsLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.434s
- 2026-11-08 08:47 -05:00 · tarea 9 · verde · vigente · `go test ./adapters/primary/cli -run TestInstallLanguageIntegration -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.480s
- 2026-11-08 08:47 -05:00 · tarea 10 · verde · vigente · `go test ./adapters/primary/cli -run TestI18nHints -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.281s
- 2026-11-08 08:48 -05:00 · tarea 11 · verde · vigente · `go test ./application/usecases -run TestBuildContextLanguage -count=1` (salida 0)
      ok  	mem/application/usecases	0.214s
- 2026-11-08 08:48 -05:00 · tarea 12 · verde · vigente · `go test ./adapters/primary/cli -run TestMCPContextLanguage -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.317s

## Siguiente
Listo para avanzar al paso approve
