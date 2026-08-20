package cli

import (
	"flag"
	"fmt"
	"os"

	"mem/application/usecases"
)

// CmdConsolidate implementa `mem consolidate`: funde memorias redundantes
// del proyecto —por clave de tópico y por registros de actividad
// idénticos— en su fila más reciente (feature 020, fase B). Previsualiza por
// defecto (FR-027, la operación es irreversible); --apply confirma.
func CmdConsolidate(deps *Deps, args []string) {
	fs := flag.NewFlagSet("consolidate", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "Aplicar de verdad (por defecto solo previsualiza)")
	if err := fs.Parse(args); err != nil {
		return
	}

	report, err := usecases.ConsolidateMemories(deps.MemoryRepo, deps.Project, *apply)
	if err != nil {
		fail("consolidar memorias: %v", err)
	}

	if len(report.Groups) == 0 {
		fmt.Println("No hay memorias consolidables (ningún grupo por clave de tópico ni por actividad duplicada).")
		return
	}

	fmt.Fprint(os.Stdout, FormatConsolidationReport(report, *apply))
}

// FormatConsolidationReport renderiza el resultado de ConsolidateMemories en
// texto plano.
func FormatConsolidationReport(report usecases.ConsolidationReport, applied bool) string {
	var out string
	verb := "se consolidarían"
	if applied {
		verb = "se consolidaron"
	}
	for _, g := range report.Groups {
		key := g.Key
		if g.Criterion == "checkpoint_duplicate" && len(key) > 12 {
			key = key[:12] // hash sha256 completo no aporta al usuario; el prefijo basta para diferenciar grupos
		}
		out += fmt.Sprintf("[%s] %s — %d filas → 1 (conserva id=%d)\n", g.Criterion, key, len(g.Memories), g.Memories[len(g.Memories)-1].ID)
	}
	out += fmt.Sprintf("\n%d grupo(s), %d fila(s) %s.\n", len(report.Groups), report.DeletedCount, verb)
	if !applied {
		out += "Previsualización: nada se modificó. Repite con --apply para aplicarlo de verdad.\n"
	}
	return out
}
