package main

import (
	"fmt"
	"path/filepath"

	"mem/store"
)

func cmdProject(args []string) {
	root, err := store.FindRoot()
	if err != nil {
		fail("no se encontró proyecto gomemory: %v\nEjecuta 'mem init' para inicializar uno", err)
	}

	project := filepath.Base(root)
	dbPath := store.DbPath(root)

	fmt.Printf("Proyecto: %s\n", project)
	fmt.Printf("Raíz:     %s\n", root)
	fmt.Printf("BD:       %s\n", dbPath)

	db, err := store.Open(root)
	if err != nil {
		return
	}
	defer db.Close()

	count := 0
	store.ListMemories(db, project, 1)
	if mems, err := store.ListMemories(db, project, 1); err == nil {
		count = len(mems)
		if count > 0 {
			if all, err := store.ListMemories(db, project, 200); err == nil {
				count = len(all)
			}
		}
	}

	// Try to get actual count
	allMems, _ := store.ListMemories(db, project, 200)
	count = len(allMems)

	fmt.Printf("Memorias:  %d\n", count)

	sess, _ := store.ActiveSession(db, project)
	if sess != nil {
		fmt.Printf("Sesión:    Activa desde %s\n", sess.CreatedAt)
	} else {
		fmt.Println("Sesión:    Ninguna activa")
	}
}
