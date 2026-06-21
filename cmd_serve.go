package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"mem/internal/server"
	"mem/store"
)

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 9735, "Puerto del servidor HTTP")
	fs.Parse(args)

	root, err := store.FindRoot()
	if err != nil {
		fail("no hay .memory/ en este proyecto. Ejecuta 'mem init' primero")
	}

	db, err := store.Open(root)
	if err != nil {
		fail("error al abrir base de datos: %v", err)
	}
	defer db.Close()

	project := filepath.Base(root)

	srv := server.New(db, project, *port)
	fmt.Printf("🧠 gomemory server corriendo en 127.0.0.1:%d\n", *port)
	fmt.Printf("   Proyecto: %s\n", project)

	if err := srv.Start(); err != nil {
		fail("servidor HTTP: %v", err)
	}
}
