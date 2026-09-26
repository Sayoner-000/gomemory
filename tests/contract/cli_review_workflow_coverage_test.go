package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIReview_CrearConsultarYMostrarRevisionDeSoloLectura(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("review")
	if res := s.Run("", project, "init"); res.ExitCode != 0 {
		t.Fatalf("init: %+v", res)
	}
	mustWrite(t, filepath.Join(project, "objetivo.txt"), "Contenido a revisar\n")
	created := s.Run("", project, "review", "--read-only", "--file", "objetivo.txt")
	if created.ExitCode != 0 || !strings.Contains(created.Stdout, "fix_authorized: false") {
		t.Fatalf("crear revisión: %+v", created)
	}
	id := strings.Fields(created.Stdout)[0]
	for _, args := range [][]string{
		{"review", "status"},
		{"review", "status", id},
		{"review", "history"},
		{"review", "history", "--limit", "1"},
		{"review", "show", id},
	} {
		res := s.Run("", project, args...)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, id) {
			t.Fatalf("review %v: %+v", args, res)
		}
	}
	bad := s.Run("", project, "review", "history", "--limit", "0")
	if bad.ExitCode == 0 {
		t.Fatalf("límite inválido aceptado: %+v", bad)
	}
}
