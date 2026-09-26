package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

func TestParseInstallArgs_FlagsEnCualquierPosicion(t *testing.T) {
	o, err := parseInstallArgs([]string{"proyecto", "--scope=global", "-y", "--agents", "claude,codex"})
	if err != nil {
		t.Fatal(err)
	}
	if o.target != "proyecto" || o.scope != "global" || !o.yes || strings.Join(o.agents, ",") != "claude,codex" {
		t.Fatalf("opciones incorrectas: %+v", o)
	}
	for _, args := range [][]string{{"--agents"}, {"--scope", "otro"}, {"--agents", "desconocido"}, {"--flag-inexistente"}} {
		if _, err := parseInstallArgs(args); err == nil {
			t.Errorf("%v debía fallar antes de instalar", args)
		}
	}
}

func TestParseUninstallArgs_AlcanceExportacionYConflictos(t *testing.T) {
	o, err := parseUninstallArgs([]string{"proyecto", "--all", "--scan=/raiz", "--export", "/salida", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	if o.target != "proyecto" || o.scope != domain.UninstallSystem || o.scanRoot != "/raiz" || o.exportDir != "/salida" || o.memory != domain.MemoryExport || !o.yes {
		t.Fatalf("opciones incorrectas: %+v", o)
	}
	for _, args := range [][]string{{"--export"}, {"--scan"}, {"--keep-memory", "--export=salida"}, {"--flag-inexistente"}} {
		if _, err := parseUninstallArgs(args); err == nil {
			t.Errorf("%v debía fallar antes de desinstalar", args)
		}
	}
}

func TestResolveInstallSelection_PrioridadYSeleccionVacia(t *testing.T) {
	choices := choicesFor("claude", "codex")
	if got := resolveInstallSelection(installOptions{}, installSelection{}, choices); strings.Join(got.agents, ",") != "claude,codex" || got.scope != "project" {
		t.Fatalf("sin selección guardada se usan los detectados: %+v", got)
	}
	saved := installSelection{agents: []string{"claude"}, scope: "global"}
	if got := resolveInstallSelection(installOptions{agents: []string{"codex"}, scope: "project"}, saved, choices); strings.Join(got.agents, ",") != "codex" || got.scope != "project" {
		t.Fatalf("los flags deben prevalecer: %+v", got)
	}
	if got := resolveInstallSelection(installOptions{}, installSelection{scope: "project"}, choices); len(got.agents) != 0 {
		t.Fatalf("ningún agente guardado debe continuar vacío: %+v", got)
	}
}

type lifecycleReleaseStub struct {
	tag, etag string
	err       error
	wantETag  string
}

func (r *lifecycleReleaseStub) Latest(_ context.Context, etag string) (string, string, bool, error) {
	r.wantETag = etag
	return r.tag, r.etag, r.tag == "", r.err
}

func (*lifecycleReleaseStub) Checksum(context.Context, string, string) (string, error) {
	return "", nil
}

type lifecycleUpdateRepoStub struct {
	read, written domain.UpdateCheck
	writes        int
}

func (r *lifecycleUpdateRepoStub) Read(context.Context) (domain.UpdateCheck, bool) {
	return r.read, true
}

func (r *lifecycleUpdateRepoStub) Write(_ context.Context, c domain.UpdateCheck) error {
	r.written = c
	r.writes++
	return nil
}

func TestRefreshUpdateCache_ConservaLoConocidoYRegistraFallos(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := &lifecycleUpdateRepoStub{read: domain.UpdateCheck{Latest: "v2.26.4", ETag: "etag-1"}}
	rel := &lifecycleReleaseStub{err: errors.New("sin red")}
	got := refreshUpdateCache(context.Background(), rel, repo, now)
	if rel.wantETag != "etag-1" || got.Latest != "v2.26.4" || got.ETag != "etag-1" || got.LastError != "sin red" || repo.writes != 1 || repo.written != got {
		t.Fatalf("fallo debe conservar la caché y escribirse: got=%+v repo=%+v", got, repo)
	}
	repo.read = got
	rel.err, rel.tag, rel.etag = nil, "", ""
	got = refreshUpdateCache(context.Background(), rel, repo, now.Add(time.Hour))
	if got.Latest != "v2.26.4" || got.ETag != "etag-1" || got.LastError != "" || repo.writes != 2 {
		t.Fatalf("304 debe conservar versión y limpiar error: %+v", got)
	}
	repo.read = got
	rel.tag, rel.etag = "v2.27.0", "etag-2"
	got = refreshUpdateCache(context.Background(), rel, repo, now.Add(2*time.Hour))
	if got.Latest != "v2.27.0" || got.ETag != "etag-2" || !got.CheckedAt.Equal(now.Add(2*time.Hour)) || repo.writes != 3 {
		t.Fatalf("200 debe actualizar la caché: %+v", got)
	}
}
