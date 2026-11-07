package usecases_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// HCD-043: una comprobación cuya salida abre <private> fuera del extracto no
// deja el contenido privado ni en la evidencia guardada ni en el documento.
func TestHCDVerifyNoGuardaTextoPrivadoRecortado(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el comando usa printf de sh; en Windows Run delega en cmd /C")
	}
	const canary = "SYNTH-PRIVATE-0043"
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Evidencia privada", Why: "fix claro", Intent: "La evidencia no guarda texto privado"})
	if err != nil {
		t.Fatal(err)
	}
	editDoc(t, env, view, commonFill...)
	stage(t, env, domain.StageImplement, "hcd-implement")

	command := `printf '<private>'; printf '%2500s' '` + canary + `'; printf '</private>\npublico-final\n'`
	view, ev, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: command})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ev.Excerpt, canary) {
		t.Fatalf("la evidencia devuelta contiene el canario: %q", ev.Excerpt)
	}
	var stored string
	if err := env.db.QueryRow(`SELECT excerpt FROM hcd_evidence WHERE id = ?`, ev.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, canary) {
		t.Fatalf("la evidencia guardada contiene el canario: %q", stored)
	}
	doc, err := os.ReadFile(filepath.Join(env.root, view.Unit.DocPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), canary) {
		t.Fatalf("el documento contiene el canario:\n%s", doc)
	}
}
