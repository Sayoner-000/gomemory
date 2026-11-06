package main

import (
	"testing"

	"mem/domain"
)

// HCD-036 tarea 8: el contenedor cablea la conversación del proceso en la
// sesión, las entregas y los bloques. Dos procesos (dos contenedores) con
// conversaciones distintas no comparten sesión, entregas ni procedencia.
func TestContenedorSesionPorConversacion(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	abrir := func(conv string) *Container {
		c, err := NewContainer(root, "hook")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		d := c.ToDeps()
		if d.Conversation == nil {
			t.Fatal("el contenedor debe cablear la conversación del proceso")
		}
		d.Conversation.Adopt(conv)
		return c
	}
	a, b := abrir("agente-A"), abrir("agente-B")
	da, db := a.ToDeps(), b.ToDeps()

	sa, err := da.SessionRepo.Start(a.Project)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := db.SessionRepo.Active(b.Project); got != nil {
		t.Fatalf("B no ve la sesión de A: %+v", got)
	}
	if _, err := db.SessionRepo.Start(b.Project); err != nil {
		t.Fatal(err)
	}
	if err := da.DeliveryLog.Record("context", "hash-A"); err != nil {
		t.Fatal(err)
	}
	if _, ok := db.DeliveryLog.Last("context"); ok {
		t.Fatal("B no ve la entrega de A")
	}
	// Q3: «Sesión Activa» de A es la suya aunque B abrió una después.
	if got, _ := da.SessionRepo.Active(a.Project); got == nil || got.ID != sa.ID {
		t.Fatalf("A conserva su sesión activa: %+v", got)
	}
	// C-03: la procedencia de una memoria de A sale del prompt de A.
	_ = da.SessionRepo.SetLastPrompt(a.Project, "prompt de A")
	_ = db.SessionRepo.SetLastPrompt(b.Project, "prompt de B")
	id, err := da.MemoryRepo.Insert(&domain.Memory{Project: a.Project, SessionID: sa.ID, Type: domain.Decision, Title: "decisión de A", Content: "contenido de la decisión de A"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := da.MemoryRepo.Get(a.Project, id)
	if err != nil || m == nil || m.OriginPrompt != "prompt de A" {
		t.Fatalf("C-03: la memoria de A lleva el prompt de A: %+v %v", m, err)
	}
}
