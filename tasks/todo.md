# Plan: Reemplazar scroll manual por `list` de charmbracelet/bubbles

## Estado
- [x] Tarea 1: Crear tipo `memoryItem` — ✅ implementado
- [x] Tarea 2: Crear Delegate custom (`memoryDelegate`) — ✅ implementado
- [x] Tarea 3: Actualizar struct `model` — ✅ list.Model agregado, cursor/searching/search eliminados
- [x] Tarea 4: Inicializar list en `initialModel` — ✅ implementado
- [x] Tarea 5: Actualizar `Update` para routing de teclas — ✅ `updateList` reescrito
- [x] Tarea 6: Reemplazar `listView()` por `m.list.View()` — ✅ implementado
- [ ] Tarea 7: Eliminar código muerto — ⚠️ windowLines/bodyBudget se conservaron para optimize
- [x] Tarea 8: Actualizar tests existentes — ✅ reescritos con `newTestModel`
- [x] Tarea 9: Verificar que compila y tests pasan — ✅ todos los tests pasan
- [ ] Tarea 10: Commit, tag, push — pendiente (v1.28.0)

## Cambios realizados
- `memoryItem`: implementa `list.Item` con `Title()`, `Description()`, `FilterValue()`
- `memoryDelegate`: implementa `list.ItemDelegate` con render custom preservando estilos
- `model`: `list list.Model` agregado, `cursor int` / `searching bool` / `search string` eliminados
- `initialModel`: inicializa list con `SetShowTitle(false)`, `SetFilteringEnabled(true)`, `DisableQuitKeybindings()`
- `updateList`: maneja `q`/`ctrl+c` (quit), `enter` (detail), `s`/`a`/`m`/`c`/`o` (custom), default → list
- `listView`: usa `m.list.SetItems(items)` + `m.list.View()`
- `visibleMemories`: eliminado (reemplazado por filtering del list)
- Tests reescritos con `newTestModel(mems, height)` helper que inicializa list.Model

## Notas
- `windowLines` / `bodyBudget` se conservan como helpers para pantallas de optimización (screenOptimize, screenOptimizeDetail)
- Test de filtrado removido: bubbles' filtering es interno y no testeable con keystrokes simulados
