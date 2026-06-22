package cli

import (
	"fmt"

	"github.com/dustin/go-humanize"
)

// FormatCompactResult arma el mensaje de resultado de `mem compact` (FR-007):
// reporta tamaño antes/después, o informa explícitamente que no había
// espacio significativo que liberar (Acceptance Scenario US2.2).
func FormatCompactResult(before, after int64) string {
	if before == after {
		return fmt.Sprintf("La base de datos ya estaba compacta (%s) — nada que liberar.", humanize.Bytes(uint64(before)))
	}
	freed := before - after
	return fmt.Sprintf("Compactado: %s → %s (liberado: %s)",
		humanize.Bytes(uint64(before)), humanize.Bytes(uint64(after)), humanize.Bytes(uint64(freed)))
}

func CmdCompact(deps *Deps, args []string) {
	before, after, err := deps.MaintenanceRepo.Compact()
	if err != nil {
		fail("compactar: %v", err)
	}
	fmt.Println(FormatCompactResult(before, after))
}
