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
