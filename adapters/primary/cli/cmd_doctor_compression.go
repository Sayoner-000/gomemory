package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// doctorCompressionJSON es la sección «Compresión» de `mem doctor` (feature
// 033, FR-029), también bajo la clave "compression" de --json.
type doctorCompressionJSON struct {
	Level          string            `json:"level"`
	Origin         string            `json:"origin"`
	OriginalsBytes int64             `json:"originals_bytes"`
	OriginalsMax   int64             `json:"originals_max_bytes"`
	OriginalsRefs  int               `json:"originals_refs"`
	NearLimit      bool              `json:"near_limit"`
	StoreWritable  bool              `json:"store_writable"`
	ToolOutput     map[string]string `json:"tool_output_hooks"`
	Tuning         []string          `json:"tuning"`
	Problems       []string          `json:"problems"`
}

// Estado del hook de salidas por runtime. Los valores de Codex y OpenCode
// reflejan lo verificado contra los binarios (research R11, tarea T062).
func toolOutputHookStates(root string, enabled bool) map[string]string {
	states := map[string]string{"claude": "inactivo", "codex": "inactivo", "opencode": "inactivo"}
	if !enabled {
		return states
	}
	if data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json")); err == nil && strings.Contains(string(data), "hook tool-output claude") {
		states["claude"] = "activo"
	} else {
		states["claude"] = "no registrado (ejecuta mem install)"
	}
	if home, err := os.UserHomeDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err == nil && strings.Contains(string(data), "hook tool-output codex") {
			states["codex"] = "parcial (solo herramientas MCP)"
		}
	}
	states["opencode"] = "v1: sin verificar en vivo · v2: no soportado"
	return states
}

func buildDoctorCompression(deps *Deps, root string) doctorCompressionJSON {
	var st ports.SettingsData
	if deps.SettingsRepo != nil {
		st = deps.SettingsRepo.Read(root)
	}
	out := doctorCompressionJSON{
		Level:      domain.ParseCompressionLevel(st.ContextCompressionLevel, st.ContextCompressionDisabled),
		ToolOutput: toolOutputHookStates(root, st.ToolOutputCompression),
		Tuning:     []string{},
		Problems:   []string{},
	}
	switch {
	case st.ContextCompressionLevel != "":
		out.Origin = "ajuste"
	case st.ContextCompressionDisabled:
		out.Origin = "heredado (context_compression_disabled)"
	default:
		out.Origin = "por defecto"
	}
	out.OriginalsMax = int64(domain.CompressionOriginalsMaxBytes)
	if st.CompressionOriginalsMaxMB > 0 {
		out.OriginalsMax = int64(st.CompressionOriginalsMaxMB) << 20
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*domain.StoreTimeout)
	defer cancel()
	if deps.OriginalStore != nil {
		if b, n, err := deps.OriginalStore.Usage(ctx); err == nil {
			out.OriginalsBytes, out.OriginalsRefs = b, n
			out.NearLimit = b*10 >= out.OriginalsMax*9
		}
		// Purge escribe (borra caducados): si funciona, el almacén es escribible.
		if _, _, err := deps.OriginalStore.Purge(ctx); err == nil {
			out.StoreWritable = true
		}
	}
	if out.Level == domain.CompressionLevelMax && !out.StoreWritable {
		out.Problems = append(out.Problems, "nivel max con el almacén de originales no escribible: toda compresión degradará a structural")
	}
	if out.NearLimit {
		out.Problems = append(out.Problems, "almacén de originales al 90 % del tope: ejecuta mem pack purge o sube compression_originals_max_mb")
	}
	if deps.CompressionTuning != nil {
		if list, err := deps.CompressionTuning.List(ctx, deps.Project); err == nil {
			for _, e := range list {
				out.Tuning = append(out.Tuning, fmt.Sprintf("%s: agresividad %d (%s)", e.ContentType, e.Aggressiveness, e.Reason))
			}
		}
	}
	return out
}

func printDoctorCompression(c doctorCompressionJSON) {
	fmt.Println("\nCompresión:")
	fmt.Printf("  nivel: %s (%s)\n", c.Level, c.Origin)
	fmt.Printf("  originales: %.1f MB de %.0f MB · %d refs · escribible: %v\n", float64(c.OriginalsBytes)/(1<<20), float64(c.OriginalsMax)/(1<<20), c.OriginalsRefs, c.StoreWritable)
	for _, rt := range []string{"claude", "codex", "opencode"} {
		fmt.Printf("  hook de salidas (%s): %s\n", rt, c.ToolOutput[rt])
	}
	for _, t := range c.Tuning {
		fmt.Println("  ajuste adaptativo: " + t)
	}
	for _, p := range c.Problems {
		fmt.Println("  ⚠ " + p)
	}
}
