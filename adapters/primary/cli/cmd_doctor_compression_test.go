package cli

import (
	"context"
	"errors"
	"testing"

	"mem/application/ports"
)

type storeDoctor struct {
	bytes    int64
	purgeErr error
}

func (s storeDoctor) Put(context.Context, string, string, string, string) (string, error) {
	return "", nil
}
func (s storeDoctor) Get(context.Context, string) (string, ports.OriginalMeta, bool, error) {
	return "", ports.OriginalMeta{}, false, nil
}
func (s storeDoctor) Purge(context.Context) (int, int64, error) { return 0, 0, s.purgeErr }
func (s storeDoctor) Usage(context.Context) (int64, int, error) { return s.bytes, 3, nil }

// T072 — sección Compresión de mem doctor.
func TestDoctorCompression(t *testing.T) {
	root := t.TempDir()
	deps := &Deps{SettingsRepo: &memSettingsRepo{s: ports.SettingsData{ContextCompressionLevel: "max", CompressionOriginalsMaxMB: 1}}, OriginalStore: storeDoctor{bytes: 1 << 19}}
	c := buildDoctorCompression(deps, root)
	if c.Level != "max" || c.Origin != "ajuste" || !c.StoreWritable || len(c.Problems) != 0 || c.OriginalsRefs != 3 {
		t.Errorf("estado sano inesperado: %+v", c)
	}
	deps.OriginalStore = storeDoctor{bytes: 1 << 19, purgeErr: errors.New("readonly")}
	if c := buildDoctorCompression(deps, root); c.StoreWritable || len(c.Problems) != 1 {
		t.Errorf("max con almacén no escribible debe contar como problema (--strict): %+v", c)
	}
	deps.OriginalStore = storeDoctor{bytes: 950 << 10}
	if c := buildDoctorCompression(deps, root); !c.NearLimit || len(c.Problems) != 1 {
		t.Errorf("al 90 %% debe avisar: %+v", c)
	}
	deps.SettingsRepo = &memSettingsRepo{s: ports.SettingsData{ContextCompressionDisabled: true}}
	deps.OriginalStore = storeDoctor{purgeErr: errors.New("x")}
	if c := buildDoctorCompression(deps, root); c.Level != "none" || len(c.Problems) != 0 || c.ToolOutput["claude"] != "inactivo" {
		t.Errorf("none con almacén caído no es un problema: %+v", c)
	}
}
