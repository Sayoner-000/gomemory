package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// CmdADRSync es `mem adr-sync status`: solo lectura, no expuesto vía MCP
// (mismo criterio que `mem purge`/`mem gc` — diagnóstico humano, no una tool
// que un agente deba invocar en medio de una tarea). Cubre FR-010: hace
// consultable el estado de sincronización sin necesidad de mirar la base
// directamente.
func CmdADRSync(deps *Deps, args []string) {
	if len(args) == 0 || args[0] != "status" {
		fail("uso: mem adr-sync status")
	}

	if deps.ADRSyncRepo == nil {
		fmt.Println("Sincronización de ADR: sin repositorio configurado.")
		return
	}

	recs, err := deps.ADRSyncRepo.ListByProject(deps.Project)
	if err != nil {
		fail("listar sincronización de ADR: %v", err)
	}
	if len(recs) == 0 {
		fmt.Println("Sin registros de sincronización de ADR todavía.")
		if deps.ADRSyncProvider == nil {
			fmt.Println("(la sincronización está desactivada: mem settings --adr-sync=true)")
		}
		return
	}

	var ok, pending, failed, conflict int
	for _, r := range recs {
		switch r.Status {
		case "ok":
			ok++
		case "pending":
			pending++
		case "failed":
			failed++
		case "conflict_resolved":
			conflict++
		}
	}
	fmt.Printf("ADR sincronizados: %d ok · %d pendiente(s) · %d fallido(s) · %d conflicto(s) resuelto(s)\n\n", ok, pending, failed, conflict)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, r := range recs {
		arrow := "→"
		if r.Origin == "provider" {
			arrow = "←"
		}
		memID := "-"
		if r.MemoryID != nil {
			memID = fmt.Sprintf("%d", *r.MemoryID)
		}
		fmt.Fprintf(w, "[%s]\t%s\t%s %s\t%s\t%s\n", memID, r.Section, arrow, r.Provider, r.Status, r.LastSyncedAt)
	}
	w.Flush()
}
