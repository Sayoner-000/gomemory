package persistence

import (
	"testing"

	"mem/domain"
)

// R12 (feature 034): la desinstalación exporta proyectos del almacén cuya ruta
// no se conoce; OpenByKey abre su base solo con la clave.
func TestOpenByKey(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, t.TempDir())
	root := t.TempDir()
	key := ProjectKey(root)

	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewMemoryRepository(db).Insert(&domain.Memory{Project: key, Title: "t", Type: domain.MemoryType("learning"), Content: "c"}); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	_ = db.Close()

	byKey, err := OpenByKey(key)
	if err != nil || byKey == nil {
		t.Fatalf("OpenByKey(%q) = %v, %v", key, byKey, err)
	}
	defer func() { _ = byKey.Close() }()
	mems, err := NewMemoryRepository(byKey).ListAll(key)
	if err != nil || len(mems) != 1 {
		t.Fatalf("memorias por clave = %d, %v", len(mems), err)
	}
}

// Ausencia esperada: nil sin error (constitución §3).
func TestOpenByKey_Inexistente(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, t.TempDir())
	db, err := OpenByKey("no-existe-0123456789abcdef")
	if db != nil || err != nil {
		t.Fatalf("OpenByKey inexistente = %v, %v; quiero nil, nil", db, err)
	}
}
