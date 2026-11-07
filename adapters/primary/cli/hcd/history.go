package hcd

import (
	"context"
	"fmt"
	"io"

	"mem/adapters/primary/cli/clikit"
)

// executeHistory atiende mem hcd index y mem hcd history (HCD-046): el
// historial de decisiones no pertenece a una unidad, así que no pasa por la
// vista de la unidad activa.
func executeHistory(deps *clikit.Deps, p parsed, out, errOut io.Writer) int {
	switch p.command {
	case "index":
		if deps == nil || deps.HistoryIndex == nil {
			return emitError(out, errOut, p.json, "internal", "el historial de decisiones no está conectado")
		}
		if p.id != "" {
			return emitError(out, errOut, p.json, "usage", "index no recibe unidad")
		}
		apply := p.flags["apply"] == "true"
		rep, err := deps.HistoryIndex.Run(context.Background(), apply)
		if err != nil {
			return emitError(out, errOut, p.json, errorCode(err), err.Error())
		}
		data := map[string]any{"applied": rep.Applied, "units": rep.Units, "audits": rep.Audits, "created": rep.Created, "updated": rep.Updated, "unchanged": rep.Unchanged, "deleted": rep.Deleted, "deleted_ids": rep.DeletedIDs}
		if p.json {
			return emitOK(out, data)
		}
		head := "Vista previa (no se escribió nada; aplica con mem hcd index --apply)"
		if apply {
			head = "Historial indexado"
		}
		fmt.Fprintf(out, "%s\n  unidades %d · auditorías %d\n  fichas nuevas %d · actualizadas %d · sin cambios %d\n  memorias de pasos retiradas %d\n", head, rep.Units, rep.Audits, rep.Created, rep.Updated, rep.Unchanged, rep.Deleted)
		return 0
	default:
		if deps == nil || deps.HistoryQuery == nil {
			return emitError(out, errOut, p.json, "internal", "el historial de decisiones no está conectado")
		}
		items, err := deps.HistoryQuery.List(p.id, p.flags["file"])
		if err != nil {
			return emitError(out, errOut, p.json, errorCode(err), err.Error())
		}
		if p.json {
			return emitOK(out, map[string]any{"items": items})
		}
		if len(items) == 0 {
			fmt.Fprintln(out, "Sin fichas en el historial (indexa con mem hcd index --apply)")
			return 0
		}
		for _, it := range items {
			fmt.Fprintf(out, "%s  %s → get_memory %d\n", it.Date, it.Title, it.ID)
		}
		return 0
	}
}

func emitOK(out io.Writer, data map[string]any) int {
	if err := emitJSON(out, Response{ProtocolVersion: ProtocolVersion, OK: true, Data: data}); err != nil {
		return 1
	}
	return 0
}
