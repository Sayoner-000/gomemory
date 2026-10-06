package cli

import (
	"context"
	"os"
	"time"

	"mem/adapters/secondary/persistence"
)

// Sección "Versión y binario" de `mem doctor` (feature 034, FR-033).

type doctorLocalCopyJSON struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type doctorUpdateCheckJSON struct {
	Enabled bool `json:"enabled"`
	// DisabledBy es la razón cuando Enabled es false: la variable o el ajuste.
	DisabledBy string `json:"disabled_by,omitempty"`
	CheckedAt  string `json:"checked_at,omitempty"`
	Latest     string `json:"latest,omitempty"`
	Error      string `json:"error,omitempty"`
}

type doctorBinaryJSON struct {
	GlobalPath    string                `json:"global_path"`
	GlobalVersion string                `json:"global_version"`
	LocalCopies   []doctorLocalCopyJSON `json:"local_copies"`
	UpdateCheck   doctorUpdateCheckJSON `json:"update_check"`
}

func buildDoctorBinary(deps *Deps, root string) doctorBinaryJSON {
	out := doctorBinaryJSON{LocalCopies: []doctorLocalCopyJSON{}}
	if g, ok := resolveGlobalBinary(root); ok {
		out.GlobalPath = g
		out.GlobalVersion, _ = identifyVersion(g)
	}

	roots := []string{}
	if root != "" {
		roots = append(roots, root)
	}
	if regs, err := persistence.ListRegisteredProjects(); err == nil {
		for _, r := range regs {
			if r.Exists && r.Root != root {
				roots = append(roots, r.Root)
			}
		}
	}
	for _, r := range roots {
		if c, ok := identifyLocalCopy(r); ok && !sameFilePath(c.Path, out.GlobalPath) {
			out.LocalCopies = append(out.LocalCopies, doctorLocalCopyJSON(c))
		}
	}

	uc := doctorUpdateCheckJSON{Enabled: !updateCheckDisabled(root)}
	if !uc.Enabled {
		uc.DisabledBy = "ajuste update_check_disabled"
		if v := os.Getenv("GOMEMORY_NO_UPDATE_CHECK"); v != "" && v != "0" && v != "false" {
			uc.DisabledBy = "GOMEMORY_NO_UPDATE_CHECK"
		}
	}
	if c, ok := updateCheckRepoOf(deps).Read(context.Background()); ok {
		uc.Latest, uc.Error = c.Latest, c.LastError
		if !c.CheckedAt.IsZero() {
			uc.CheckedAt = c.CheckedAt.Format(time.RFC3339)
		}
	}
	out.UpdateCheck = uc
	return out
}

func printDoctorBinary(b doctorBinaryJSON) {
	humanln("\nVersión y binario:")
	if b.GlobalPath == "" {
		humanln("  binario global: no hay `mem` en el PATH (los proyectos usan su copia)")
	} else {
		humanf("  binario global: %s (%s)\n", b.GlobalPath, b.GlobalVersion)
	}
	if len(b.LocalCopies) == 0 {
		humanln("  copias locales: ninguna")
	}
	for _, c := range b.LocalCopies {
		humanf("  copia local: %s (%s) — se retirará al abrir el proyecto\n", c.Path, c.Version)
	}
	if b.UpdateCheck.Enabled {
		humanln("  aviso de versión: activo")
	} else {
		humanf("  aviso de versión: desactivado (%s)\n", b.UpdateCheck.DisabledBy)
	}
	switch {
	case b.UpdateCheck.CheckedAt == "":
		humanln("  última consulta: nunca")
	case b.UpdateCheck.Error != "":
		humanf("  última consulta: %s → error: %s\n", b.UpdateCheck.CheckedAt, b.UpdateCheck.Error)
	default:
		humanf("  última consulta: %s → %s\n", b.UpdateCheck.CheckedAt, b.UpdateCheck.Latest)
	}
}
