package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"mem/domain"
)

func TestWriteFileAtomic_ConcurrenteSiempreLegible(t *testing.T) {
	p := filepath.Join(t.TempDir(), "contador")
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if err := writeFileAtomic(p, []byte(strconv.Itoa(g*1000+i)), 0o600); err != nil {
					t.Errorf("writeFileAtomic: %v", err)
					return
				}
				if raw, err := os.ReadFile(p); err == nil {
					if _, err := strconv.Atoi(strings.TrimSpace(string(raw))); err != nil {
						t.Errorf("lectura concurrente corrupta: %q", raw)
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("leer: %v", err)
	}
	if _, err := strconv.Atoi(string(raw)); err != nil {
		t.Fatalf("el archivo final debe ser un entero válido: %q", raw)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("no deben quedar temporales: %v", entries)
	}
}

func TestConversation_IdaYVuelta(t *testing.T) {
	root := t.TempDir()
	if err := writeConversation(root, domain.Conversation{ID: "A", StartedAt: 42}); err != nil {
		t.Fatalf("writeConversation: %v", err)
	}
	got, ok := readConversation(root)
	if !ok || got.ID != "A" || got.StartedAt != 42 {
		t.Errorf("ida y vuelta: %+v %v", got, ok)
	}
	info, err := os.Stat(conversationPath(root))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permisos: %o, se esperaba 600", perm)
	}
}

func TestConversation_CorruptaEsAusenteYSeBorra(t *testing.T) {
	root := t.TempDir()
	p := conversationPath(root)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte("{no es json"), 0o600); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	if _, ok := readConversation(root); ok {
		t.Error("un archivo corrupto debe leerse como ausente")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("el archivo corrupto debía borrarse")
	}
}
