package cli

import (
	"fmt"
	"os"
	"strings"
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
		humanln("Sincronización de ADR: sin repositorio configurado.")
		return
	}

	recs, err := deps.ADRSyncRepo.ListByProject(deps.Project)
	if err != nil {
		fail("listar sincronización de ADR: %v", err)
	}
	if len(recs) == 0 {
		humanln("Sin registros de sincronización de ADR todavía.")
		if deps.ADRSyncProvider == nil {
			humanln("(la sincronización está desactivada: mem settings --adr-sync=true)")
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
	summary := fmt.Sprintf("ADR sincronizados: %d ok · %d pendiente(s) · %d fallido(s) · %d conflicto(s) resuelto(s)", ok, pending, failed, conflict)
	rows := make([][]string, 0, len(recs))
	for _, r := range recs {
		arrow := "→"
		if r.Origin == "provider" {
			arrow = "←"
		}
		memID := "-"
		if r.MemoryID != nil {
			memID = fmt.Sprintf("%d", *r.MemoryID)
		}
		rows = append(rows, []string{"[" + memID + "]", r.Section, arrow + " " + r.Provider, string(r.Status), fmt.Sprint(r.LastSyncedAt)})
	}
	if printListPanel("", "ADR sincronizados", fmt.Sprintf("%d ok · %d pendientes · %d fallidos", ok, pending, failed), []string{"Memoria", "Sección", "Proveedor", "Estado", "Sincronizado"}, rows) {
		return
	}
	fmt.Println(summary + "\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, r := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	_ = w.Flush()
}
