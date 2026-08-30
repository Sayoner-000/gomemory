package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"mem/adapters/primary/setup"
)

// consolidateCodexHooks convierte la representación heredada hooks.json al
// dialecto TOML de Codex. No conoce comandos ni proveedores concretos: trata
// cada evento y cada grupo de hooks como datos, preserva campos desconocidos y
// elimina únicamente grupos semánticamente equivalentes.
func consolidateCodexHooks(config []byte, hooksJSON []byte) ([]byte, int, error) {
	var configDoc map[string]any
	if len(bytes.TrimSpace(config)) > 0 {
		if err := toml.Unmarshal(config, &configDoc); err != nil {
			return nil, 0, fmt.Errorf("config.toml inválido: %w", err)
		}
	} else {
		configDoc = make(map[string]any)
	}

	decoder := json.NewDecoder(bytes.NewReader(hooksJSON))
	decoder.UseNumber()
	var legacyDoc map[string]any
	if err := decoder.Decode(&legacyDoc); err != nil {
		return nil, 0, fmt.Errorf("hooks.json inválido: %w", err)
	}

	legacyHooks, ok := stringMap(legacyDoc["hooks"])
	if !ok {
		return nil, 0, fmt.Errorf("hooks.json no contiene un objeto hooks")
	}
	currentHooks, _ := stringMap(configDoc["hooks"])
	if currentHooks == nil {
		currentHooks = make(map[string]any)
	}
	delete(currentHooks, "state") // sus rutas e índices dejan de ser confiables al combinar fuentes

	migrated := 0
	for event, rawGroups := range legacyHooks {
		groups, ok := anySlice(rawGroups)
		if !ok {
			return nil, 0, fmt.Errorf("evento %q de hooks.json no es una lista", event)
		}
		current, ok := anySlice(currentHooks[event])
		if currentHooks[event] != nil && !ok {
			return nil, 0, fmt.Errorf("evento %q de config.toml no es una lista", event)
		}
		seen := make(map[string]struct{}, len(current)+len(groups))
		deduplicated := make([]any, 0, len(current)+len(groups))
		for _, group := range current {
			identity, err := hookIdentity(group)
			if err != nil {
				return nil, 0, fmt.Errorf("evento %q: %w", event, err)
			}
			if _, exists := seen[identity]; exists {
				continue
			}
			seen[identity] = struct{}{}
			deduplicated = append(deduplicated, group)
		}
		for _, legacyGroup := range groups {
			group := convertJSONNumbers(legacyGroup)
			identity, err := hookIdentity(group)
			if err != nil {
				return nil, 0, fmt.Errorf("evento %q: %w", event, err)
			}
			if _, exists := seen[identity]; exists {
				continue
			}
			seen[identity] = struct{}{}
			deduplicated = append(deduplicated, group)
			migrated++
		}
		currentHooks[event] = deduplicated
	}

	// También elimina duplicados que ya existían dentro de config.toml.
	for event, rawGroups := range currentHooks {
		groups, ok := anySlice(rawGroups)
		if !ok {
			continue
		}
		seen := make(map[string]struct{}, len(groups))
		deduplicated := make([]any, 0, len(groups))
		for _, group := range groups {
			identity, err := hookIdentity(group)
			if err != nil {
				return nil, 0, fmt.Errorf("evento %q: %w", event, err)
			}
			if _, exists := seen[identity]; exists {
				continue
			}
			seen[identity] = struct{}{}
			deduplicated = append(deduplicated, group)
		}
		currentHooks[event] = deduplicated
	}

	hooksBlock, err := toml.Marshal(map[string]any{"hooks": currentHooks})
	if err != nil {
		return nil, 0, fmt.Errorf("serializar hooks consolidados: %w", err)
	}
	base := stripCodexHooksTables(string(config))
	if base != "" && !strings.HasSuffix(base, "\n") {
		base += "\n"
	}
	candidate := []byte(strings.TrimRight(base, "\n") + "\n\n" + strings.TrimLeft(string(hooksBlock), "\n"))
	var validated map[string]any
	if err := toml.Unmarshal(candidate, &validated); err != nil {
		return nil, 0, fmt.Errorf("candidato TOML inválido: %w", err)
	}
	return candidate, migrated, nil
}

func convertJSONNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = convertJSONNumbers(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = convertJSONNumbers(typed[i])
		}
		return out
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if decimal, err := typed.Float64(); err == nil {
			return decimal
		}
		return typed.String()
	default:
		return value
	}
}

func stringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func anySlice(value any) ([]any, bool) {
	if value == nil {
		return nil, true
	}
	items, ok := value.([]any)
	return items, ok
}

func hookIdentity(value any) (string, error) {
	normalized := normalizeHookValue(value)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("no se pudo normalizar un grupo: %w", err)
	}
	return string(encoded), nil
}

func normalizeHookValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(typed))
		for _, key := range keys {
			out[key] = normalizeHookValue(typed[key])
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = normalizeHookValue(typed[i])
		}
		return out
	case json.Number:
		return "number:" + typed.String()
	}
	valueOf := reflect.ValueOf(value)
	if valueOf.IsValid() {
		switch valueOf.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return fmt.Sprintf("number:%d", valueOf.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return fmt.Sprintf("number:%d", valueOf.Uint())
		case reflect.Float32, reflect.Float64:
			return fmt.Sprintf("number:%v", value)
		}
	}
	return value
}

func stripCodexHooksTables(content string) string {
	lines := strings.SplitAfter(content, "\n")
	var out strings.Builder
	dropping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTOMLTableHeader(trimmed) {
			dropping = isCodexHooksHeader(trimmed)
		}
		if !dropping {
			out.WriteString(line)
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

func isCodexHooksHeader(header string) bool {
	trimmed := strings.TrimSpace(header)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimPrefix(trimmed, "[")
	return trimmed == "hooks]" || trimmed == "hooks]]" || strings.HasPrefix(trimmed, "hooks.")
}

func backupExistingFile(path string, data []byte, mode os.FileMode) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	return backupCodexConfig(path, data, mode)
}

func removeLegacyHooksJSON(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ensureCodexGomemoryHooks devuelve el config.toml con los enganches del ciclo
// de vida de gomemory presentes, junto al número de enganches añadidos o
// actualizados.
// Idempotente: reaplicarlo sobre su propio resultado no cambia nada.
//
// Añade al final de la lista de cada evento lo que falta y reescribe, EN SU
// SITIO, el comando de los hooks de gomemory que quedaron desalineados de la
// tabla vigente. No reordena ni retira hooks ajenos, y por eso el estado de
// confianza de los que ya estaban conserva su posición e identidad
// (contracts/hooks-config.md). Reescribir el comando sí invalida el
// trusted_hash de ESE hook, que es el precio de corregirlo: Codex vuelve a
// pedir autorización para él, y por eso el instalador lo avisa. El de los hooks
// nuevos lo genera Codex al autorizarlos — aquí jamás se calcula un
// trusted_hash a mano.
//
// El resto del archivo se preserva verbatim: se recortan únicamente las tablas
// de hooks y se anexan reserializadas, igual que hace consolidateCodexHooks.
func ensureCodexGomemoryHooks(config []byte, memCommand string) ([]byte, int, error) {
	doc := map[string]any{}
	if len(bytes.TrimSpace(config)) > 0 {
		if err := toml.Unmarshal(config, &doc); err != nil {
			return nil, 0, fmt.Errorf("config.toml inválido: %w", err)
		}
	}

	hooks, _ := stringMap(doc["hooks"])
	if hooks == nil {
		hooks = make(map[string]any)
	}

	cambiados := 0
	for _, h := range setup.CodexGomemoryHooks() {
		if setup.CodexHookPresente(hooks, h) {
			continue
		}
		grupos, _ := anySlice(hooks[h.Event])
		if replaceLegacyCodexHook(grupos, h, memCommand) {
			hooks[h.Event] = grupos
			cambiados++
			continue
		}
		hooks[h.Event] = append(grupos, setup.CodexHookGroup(h, memCommand))
		cambiados++
	}
	if cambiados == 0 {
		return config, 0, nil
	}

	bloque, err := toml.Marshal(map[string]any{"hooks": hooks})
	if err != nil {
		return nil, 0, fmt.Errorf("serializar hooks de gomemory: %w", err)
	}

	base := ensureCodexHooksFeature(stripCodexHooksTables(string(config)))
	candidate := []byte(strings.TrimRight(base, "\n") + "\n\n" + strings.TrimLeft(string(bloque), "\n"))
	var validado map[string]any
	if err := toml.Unmarshal(candidate, &validado); err != nil {
		return nil, 0, fmt.Errorf("candidato TOML inválido: %w", err)
	}
	return candidate, cambiados, nil
}

// replaceLegacyCodexHook actualiza un hook de gomemory ya instalado cuyo
// subcomando coincide pero cuyo comando quedó desalineado de la tabla vigente.
// Reemplazarlo en su grupo preserva los hooks ajenos y evita ejecutar dos veces
// el mismo evento durante una actualización.
//
// Reconcilia en LAS DOS direcciones, y no solo añadiendo el dialecto que falta.
// Un hook al que la tabla le retire el Emit —una vuelta atrás, o un subcomando
// que deje de inyectar texto al modelo— dejaría de reconocerse como presente y
// se añadiría otra vez, duplicando el evento en toda instalación ya migrada:
// exactamente el defecto que esta función existe para cerrar, en el sentido
// contrario. Por eso reescribe hacia la forma canónica sea cual sea, en vez de
// tratar el --emit como algo que solo se agrega.
func replaceLegacyCodexHook(groups []any, h setup.CodexHook, memCommand string) bool {
	for _, rawGroup := range groups {
		group, ok := stringMap(rawGroup)
		if !ok {
			continue
		}
		matcher, _ := group["matcher"].(string)
		if matcher != h.Matcher {
			continue
		}
		actions, _ := anySlice(group["hooks"])
		for _, rawAction := range actions {
			action, ok := stringMap(rawAction)
			if !ok {
				continue
			}
			command, _ := action["command"].(string)
			indice := strings.Index(command, "hook "+h.Sub)
			if indice < 0 {
				continue
			}
			// Se conserva TODO lo que precede al subcomando: quien apunta a una
			// ruta absoluta lo hace porque `mem` no está en el PATH que ve
			// Codex, y sustituirla por el comando por defecto dejaría el hook
			// registrado, visible y muerto. De este comando solo el dialecto es
			// de gomemory; la ruta es del usuario.
			binario := strings.TrimSpace(command[:indice])
			if binario == "" {
				binario = memCommand
			}
			action["command"] = setup.CodexHookCommand(h, binario)
			return true
		}
	}
	return false
}

// ensureCodexHooksFeature garantiza `[features] hooks = true`. Sin esa bandera
// Codex ignora la sección entera: dejar los hooks escritos sin activarla
// produce un ciclo presente y muerto, indistinguible de uno sano para quien
// solo mire el archivo.
//
// Inserta la clave dentro de la tabla `[features]` existente en vez de anexar
// una segunda —que rompería el TOML— y respeta el resto del archivo.
func ensureCodexHooksFeature(base string) string {
	var doc map[string]any
	if toml.Unmarshal([]byte(base), &doc) == nil {
		if features, ok := stringMap(doc["features"]); ok {
			if activo, _ := features["hooks"].(bool); activo {
				return base
			}
		}
	}

	lines := strings.SplitAfter(base, "\n")
	dentro := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTOMLTableHeader(trimmed) {
			if dentro {
				// Se sale de [features] sin haber encontrado la clave: se
				// inserta justo antes de la tabla siguiente, dentro todavía.
				lines[i] = "hooks = true\n" + line
				return strings.Join(lines, "")
			}
			dentro = trimmed == "[features]"
			continue
		}
		if !dentro {
			continue
		}
		// Reemplazar la clave existente, nunca añadir una segunda: un TOML con
		// la misma clave dos veces en la misma tabla es inválido.
		if clave, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(clave) == "hooks" {
			lines[i] = "hooks = true\n"
			return strings.Join(lines, "")
		}
	}
	if dentro {
		return strings.TrimRight(strings.Join(lines, ""), "\n") + "\nhooks = true\n"
	}

	if base != "" && !strings.HasSuffix(base, "\n") {
		base += "\n"
	}
	return base + "\n[features]\nhooks = true\n"
}
