// Package rulesfs carga las reglas de expand desde .gomemory/rules/*.md
// (feature 040, contracts/rules-format.md), con topes, saneamiento y una
// caché en .memory/ que se invalida al cambiar cualquier archivo.
package rulesfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// RulesDir es el directorio de reglas propias, relativo a la raíz. Vive
// fuera de .memory/ (ignorado por git) para que las reglas se versionen.
var RulesDir = filepath.Join(".gomemory", "rules")

const cacheFile = ".expand-rules-cache.json"

// Loader implementa ports.RuleSourcePort.
type Loader struct{}

var _ ports.RuleSourcePort = Loader{}

func New() Loader { return Loader{} }

type cacheEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

type cacheData struct {
	Entries []cacheEntry           `json:"entries"`
	Rules   []domain.Rule          `json:"rules"`
	Errors  []domain.RuleLoadError `json:"errors"`
}

// Load devuelve la regla de serie seguida de las propias, en orden de
// nombre de archivo. Ante un ctx vencido devuelve solo la de serie.
func (Loader) Load(ctx context.Context, root string) ([]domain.Rule, []domain.RuleLoadError) {
	rules := domain.BuiltinRules()
	if ctx.Err() != nil {
		return rules, nil
	}
	entries := listRuleFiles(filepath.Join(root, RulesDir))
	if len(entries) == 0 {
		return rules, nil
	}
	cachePath := filepath.Join(root, persistence.MemDir, cacheFile)
	if cached, ok := readCache(cachePath, entries); ok {
		return append(rules, cached.Rules...), cached.Errors
	}
	var loaded []domain.Rule
	var errs []domain.RuleLoadError
	for i, e := range entries {
		if ctx.Err() != nil {
			return domain.BuiltinRules(), nil
		}
		if i >= domain.RuleMaxCount {
			errs = append(errs, domain.RuleLoadError{Source: e.Name, Reason: fmt.Sprintf("se supera el máximo de %d reglas", domain.RuleMaxCount)})
			continue
		}
		if e.Size > domain.RuleFileMaxBytes {
			errs = append(errs, domain.RuleLoadError{Source: e.Name, Reason: fmt.Sprintf("supera %d bytes", domain.RuleFileMaxBytes)})
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, RulesDir, e.Name))
		if err != nil {
			errs = append(errs, domain.RuleLoadError{Source: e.Name, Reason: "no se pudo leer"})
			continue
		}
		r, err := domain.ParseRule(e.Name, raw)
		if err != nil {
			var le *domain.RuleLoadError
			if errors.As(err, &le) {
				errs = append(errs, *le)
			}
			continue
		}
		loaded = append(loaded, r)
	}
	writeCache(cachePath, cacheData{Entries: entries, Rules: loaded, Errors: errs})
	return append(rules, loaded...), errs
}

// listRuleFiles devuelve los .md regulares del directorio (sin enlaces
// simbólicos), ordenados por nombre.
func listRuleFiles(dir string) []cacheEntry {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []cacheEntry
	for _, d := range des {
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".md") {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		out = append(out, cacheEntry{Name: d.Name(), Size: info.Size(), MTime: info.ModTime().UnixNano()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func readCache(path string, entries []cacheEntry) (cacheData, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheData{}, false
	}
	var c cacheData
	if json.Unmarshal(data, &c) != nil || !reflect.DeepEqual(c.Entries, entries) {
		return cacheData{}, false
	}
	for i := range c.Rules {
		c.Rules[i].Origin = domain.RuleOriginProject
		if c.Rules[i].Compile() != nil {
			return cacheData{}, false
		}
	}
	return c, true
}

// writeCache es best-effort: sin caché, la próxima carga vuelve a parsear.
func writeCache(path string, c cacheData) {
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".expand-rules-*")
	if err != nil {
		return
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	_ = tmp.Close()
	if werr != nil || os.Chmod(name, 0o600) != nil || os.Rename(name, path) != nil {
		_ = os.Remove(name)
	}
}
