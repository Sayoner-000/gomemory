package cli

import (
	"fmt"
	"os"
	"strconv"
)

// CmdGet implementa `mem get <id>`: recupera una memoria por identificador.
// Es el canal CLI de la misma capacidad de detalle que la tool MCP
// get_memory (feature 020, FR-033) — el modo índice de get_context depende
// de que esta capacidad exista en todo canal por el que se consuma el
// índice, sin introducir ninguna nueva.
func CmdGet(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("uso: mem get <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fail("id inválido: %q", args[0])
	}

	m, err := deps.MemoryRepo.Get(deps.Project, id)
	if err != nil {
		fail("obtener memoria %d: %v", id, err)
	}
	if m == nil {
		fmt.Fprintf(os.Stderr, "Memoria %d no encontrada\n", id)
		os.Exit(1)
	}
	fmt.Println(renderMemoryDetail(*m))
}
