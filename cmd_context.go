package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"mem/context"
	"mem/store"
)

func cmdContext(args []string) {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	write := fs.Bool("w", false, "Escribir a .memory/context.md")
	fs.BoolVar(write, "write", false, "Escribir a .memory/context.md")
	fs.Parse(args)

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
	builder := context.New(db, root, project)
	output, err := builder.Build()
	if err != nil {
		fail("generar contexto: %v", err)
	}

	if *write {
		if err := builder.WriteFile(); err != nil {
			fail("escribir context.md: %v", err)
		}
		fmt.Printf("✓ Contexto escrito en %s\n", filepath.Join(root, store.MemDir, "context.md"))
	} else {
		os.Stdout.WriteString(output)
	}
}
