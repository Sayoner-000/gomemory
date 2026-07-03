package cli

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"mem/domain"
)

func CmdSave(deps *Deps, args []string) {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	title := fs.String("t", "", "Título descriptivo")
	mtype := fs.String("y", "learning", "Tipo: learning|decision|architecture|bugfix|pattern|discovery|preference")
	filepathStr := fs.String("f", "", "Archivo relacionado")
	fs.Parse(args)

	content := strings.Join(fs.Args(), " ")
	if content == "" {
		fail("el contenido es obligatorio\nEjemplo: mem save -t \"usamos SQLite\" \"Decidimos usar SQLite como base de datos\"")
	}

	memType := domain.ValidMemoryType(*mtype)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)

	var sessionID string
	sess, _ := deps.SessionRepo.Active(project)
	if sess != nil {
		sessionID = sess.ID
	}

	mem := domain.Memory{
		Project:   project,
		SessionID: sessionID,
		Type:      memType,
		Title:     *title,
		Content:   content,
		Filepath:  *filepathStr,
	}

	id, err := deps.MemoryRepo.Insert(&mem)
	if err != nil {
		fail("guardar memoria: %v", err)
	}

	fmt.Printf("✓ Memoria guardada (id=%d)\n", id)
	if sessionID != "" {
		fmt.Printf("  Sesión activa: %s\n", sessionID[:8])
	}
}
