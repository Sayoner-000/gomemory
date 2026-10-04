package cli

import (
	"fmt"
	"os"
	"strings"

	"mem/adapters/primary/setup"
	"mem/application/ports"
	"mem/domain"
)

// printDoctorHookGuard muestra los hooks de gomemory duplicados entre ámbitos
// y los conteos de las protecciones de los últimos 7 días (feature 035,
// FR-003/FR-029). Recortes y duplicados son ⚠: delatan un canal degradado. Las
// exclusiones de compresión son la protección funcionando: se cuentan sin
// alarma. Sin nada que mostrar, no imprime la sección.
func printDoctorHookGuard(dups []string, counts []domain.GuardCount) {
	if len(dups) == 0 && len(counts) == 0 {
		return
	}
	fmt.Println("\nProtecciones de hooks:")
	if len(dups) > 0 {
		fmt.Printf("  ⚠ hooks de gomemory duplicados en usuario y proyecto (claude): %s — cada evento corre dos veces → mem update\n",
			strings.Join(dups, ", "))
	}
	if len(counts) > 0 {
		fmt.Println("  protecciones (7 días):")
		for _, c := range counts {
			symbol := "·"
			if c.Kind == domain.GuardDuplicateDropped || c.Kind == domain.GuardBudgetTrimmed {
				symbol = "⚠"
			}
			detail := ""
			if c.LastDetail != "" {
				detail = "  (último: " + c.LastDetail + ")"
			}
			fmt.Printf("    %s %-9s %-21s %d%s\n", symbol, c.Agent, c.Kind, c.Count, detail)
		}
	}
}

// doctorHookGuardState reúne lo que muestra printDoctorHookGuard. Best-effort:
// sin HOME o sin registro, esa parte queda vacía.
func doctorHookGuardState(deps *Deps, root string) ([]string, []domain.GuardCount) {
	var dups []string
	if home, err := os.UserHomeDir(); err == nil && root != "" {
		dups = setup.DuplicateClaudeHookSubs(home, root)
	}
	var counts []domain.GuardCount
	if deps != nil && deps.ChannelActivity != nil {
		if rec, ok := deps.ChannelActivity.(ports.HookGuardRecorder); ok {
			counts, _ = rec.GuardSince(7)
		}
	}
	return dups, counts
}
