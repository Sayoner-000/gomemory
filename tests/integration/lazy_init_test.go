package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPStartsWithoutPriorInstall cubre US1 (specs/005-global-mcp-store):
// un repo nunca antes usado con gomemory (sin .memory/ preexistente, sin
// `mem init` previo) debe permitir que `mem mcp --root <dir>` arranque y
// responda al handshake MCP sin error — antes fallaba con
// "no existe .memory en <dir> (ejecuta 'mem init' primero)".
func TestMCPStartsWithoutPriorInstall(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir() // deliberadamente SIN persistence.EnsureDir/Init previo

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.Command(bin, "mcp", "--root", target)
	transport := &mcp.CommandTransport{Command: cmd}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("mem mcp debió arrancar sin instalación previa, pero: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("esperaba al menos una tool MCP registrada")
	}
}

// TestSaveAndSearchWithoutPriorInit cubre US1: `mem save` seguido de
// `mem search` deben funcionar en un repo `git init` recién creado, sin
// `mem init` previo — el store global se crea de forma perezosa.
func TestSaveAndSearchWithoutPriorInit(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	gitInit := exec.Command("git", "init", "-q")
	gitInit.Dir = target
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	save := exec.Command(bin, "save", "-t", "prueba lazy init", "-y", "decision", "contenido de prueba US1")
	save.Dir = target
	var saveOut bytes.Buffer
	save.Stdout = &saveOut
	save.Stderr = &saveOut
	if err := save.Run(); err != nil {
		t.Fatalf("mem save sin mem init previo debió funcionar, pero falló: %v\n%s", err, saveOut.String())
	}

	search := exec.Command(bin, "search", "lazy init")
	search.Dir = target
	var searchOut bytes.Buffer
	search.Stdout = &searchOut
	search.Stderr = &searchOut
	if err := search.Run(); err != nil {
		t.Fatalf("mem search: %v\n%s", err, searchOut.String())
	}
	if !bytes.Contains(searchOut.Bytes(), []byte("prueba lazy init")) {
		t.Fatalf("esperaba encontrar la memoria guardada, output: %s", searchOut.String())
	}
}

// TestNoRepoFilesCreated cubre SC-003 del spec: usar gomemory en un proyecto
// no debe dejar `.mcp.json`, binario copiado, ni bloques de protocolo en
// AGENTS.md/CLAUDE.md — el store de datos vive fuera del árbol del proyecto.
func TestNoRepoFilesCreated(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	save := exec.Command(bin, "save", "-t", "prueba footprint", "-y", "decision", "contenido")
	save.Dir = target
	if out, err := save.CombinedOutput(); err != nil {
		t.Fatalf("mem save: %v\n%s", err, out)
	}

	for _, artifact := range []string{".mcp.json", "mem", "AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(target, artifact)); err == nil {
			t.Fatalf("no debió crearse %q dentro del repo (SC-003)", artifact)
		}
	}
}
