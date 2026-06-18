package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"mem/store"
)

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	force := fs.Bool("force", false, "Reinicializar si ya existe")
	fs.Parse(args)

	root, err := os.Getwd()
	if err != nil {
		fail("obtener directorio actual: %v", err)
	}

	memDir := filepath.Join(root, store.MemDir)
	if _, err := os.Stat(memDir); err == nil {
		if !*force {
			fatalf("✓ .memory/ ya existe en %s (usa --force para reinicializar)", root)
			return
		}
		os.RemoveAll(memDir)
	}

	db, err := store.Init(root)
	if err != nil {
		fail("inicializar base de datos: %v", err)
	}
	db.Close()

	project := filepath.Base(root)
	fmt.Printf("✓ Memoria inicializada para proyecto '%s'\n", project)
	fmt.Printf("  Directorio: %s\n", memDir)
	fmt.Printf("  Base de datos: %s\n", store.DbPath(root))
	fmt.Println()
	fmt.Println("  Próximos pasos:")
	fmt.Println("    mem save -t \"primera entrada\" \"Aprendizaje inicial del proyecto\"")
	fmt.Println("    mem context --write    # genera .memory/context.md")
}
