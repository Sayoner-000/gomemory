package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"mem/store"
	"mem/types"
)

func cmdSave(args []string) {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	title := fs.String("t", "", "Título descriptivo")
	mtype := fs.String("y", "learning", "Tipo: learning|decision|architecture|bugfix|pattern|discovery")
	filepathStr := fs.String("f", "", "Archivo relacionado")
	fs.Parse(args)

	content := strings.Join(fs.Args(), " ")
	if content == "" {
		fail("el contenido es obligatorio\nEjemplo: mem save -t \"usamos SQLite\" \"Decidimos usar SQLite como base de datos\"")
	}

	memType := types.ValidMemoryType(*mtype)

	root, err := store.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	db, err := store.Open(root)
	if err != nil {
		fail("%v", err)
	}
	defer db.Close()

	project := filepath.Base(root)

	// Try to attach to active session
	var sessionID string
	sess, _ := store.ActiveSession(db, project)
	if sess != nil {
		sessionID = sess.ID
	}

	mem := types.Memory{
		Project:   project,
		SessionID: sessionID,
		Type:      memType,
		Title:     *title,
		Content:   content,
		Filepath:  *filepathStr,
	}

	id, err := store.InsertMemory(db, &mem)
	if err != nil {
		fail("guardar memoria: %v", err)
	}

	fmt.Printf("✓ Memoria guardada (id=%d)\n", id)
	if sessionID != "" {
		fmt.Printf("  Sesión activa: %s\n", sessionID[:8])
	}
}
