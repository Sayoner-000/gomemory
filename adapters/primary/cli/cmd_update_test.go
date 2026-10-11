package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetNameFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"linux", "amd64", "mem_linux_amd64.tar.gz"},
		{"linux", "arm64", "mem_linux_arm64.tar.gz"},
		{"darwin", "arm64", "mem_darwin_arm64.tar.gz"},
		{"darwin", "amd64", "mem_darwin_amd64.tar.gz"},
		{"windows", "amd64", "mem_windows_amd64.zip"},
		{"windows", "arm64", "mem_windows_arm64.zip"},
	}
	for _, c := range cases {
		got := assetNameFor(c.goos, c.goarch)
		if got != c.want {
			t.Errorf("assetNameFor(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestLatestReleaseTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/Sayoner-000/gomemory/releases/latest" {
			t.Errorf("ruta inesperada: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.9.0"}); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	origBase := releaseAPIBase
	releaseAPIBase = srv.URL
	defer func() { releaseAPIBase = origBase }()

	tag, err := latestReleaseTag(srv.Client())
	if err != nil {
		t.Fatalf("latestReleaseTag: %v", err)
	}
	if tag != "v1.9.0" {
		t.Errorf("esperaba v1.9.0, got %s", tag)
	}
}

func TestLatestReleaseTagErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()

	origBase := releaseAPIBase
	releaseAPIBase = srv.URL
	defer func() { releaseAPIBase = origBase }()

	if _, err := latestReleaseTag(srv.Client()); err == nil {
		t.Fatal("esperaba error ante status 404")
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "mem_linux_amd64.tar.gz")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	content := []byte("#!/bin/sh\necho fake mem binary\n")
	if err := tw.WriteHeader(&tar.Header{Name: "mem", Mode: 0755, Size: int64(len(content))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	destPath, err := extractBinary(archivePath, dir)
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Error("contenido del binario extraído no coincide")
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "mem_windows_amd64.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	content := []byte("MZ fake windows binary")
	fw, err := zw.Create("mem.exe")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	if err := extractBinaryFromZip(archivePath, "mem.exe", filepath.Join(dir, "mem.exe")); err != nil {
		t.Fatalf("extractBinaryFromZip: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "mem.exe"))
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Error("contenido del binario extraído no coincide")
	}
}

func TestExtractBinaryMissingFromArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "empty.tar.gz")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := extractBinary(archivePath, dir); err == nil {
		t.Fatal("esperaba error: el archivo no contiene el binario mem")
	}
}

func TestReplaceSelfUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replaceSelf en Windows no sobrescribe el binario en ejecución por diseño")
	}
	dir := t.TempDir()
	current := filepath.Join(dir, "mem")
	newBin := filepath.Join(dir, "mem-new")

	if err := os.WriteFile(current, []byte("old content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBin, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceSelf(current, newBin); err != nil {
		t.Fatalf("replaceSelf: %v", err)
	}

	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("esperaba 'new content', got %q", string(data))
	}
	if _, err := os.Stat(current + ".old"); err == nil {
		t.Error("el archivo .old debió limpiarse tras un reemplazo exitoso")
	}
}

// writeScript037 escribe un ejecutable de shell que imprime word.
func writeScript037(t *testing.T, path, word string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho "+word+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runScript037(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(path).Output()
	if err != nil {
		t.Fatalf("ejecutar %s: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

// TestReplaceSelfUnix_NuncaAusenteNiParcial cubre US4.4 (spec 037, H-04):
// mientras se prepara el binario nuevo, el destino sigue siendo el anterior
// completo y ejecutable; tras publicar, es el nuevo. Sin ventana de ausencia.
func TestReplaceSelfUnix_NuncaAusenteNiParcial(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el contrato de disponibilidad aplica a Unix")
	}
	dir := t.TempDir()
	current, newBin := filepath.Join(dir, "mem"), filepath.Join(dir, "mem-new")
	writeScript037(t, current, "old")
	writeScript037(t, newBin, "new")

	observed := map[string]string{}
	replaceStep = func(stage string) error {
		observed[stage] = runScript037(t, current)
		return nil
	}
	t.Cleanup(func() { replaceStep = func(string) error { return nil } })

	if err := replaceSelf(current, newBin); err != nil {
		t.Fatalf("replaceSelf: %v", err)
	}
	for _, stage := range []string{"copied", "chmod", "sync", "close", "rename"} {
		if observed[stage] != "old" {
			t.Fatalf("en la etapa %q el destino respondió %q, quiero el anterior completo", stage, observed[stage])
		}
	}
	if got := runScript037(t, current); got != "new" {
		t.Fatalf("tras publicar: %q", got)
	}
	assertNoUpdateLeftovers(t, dir, "mem", "mem-new")
}

// TestReplaceSelfUnix_FalloAntesDePublicarConservaAnterior cubre US4.5: un
// fallo o cancelación en cualquier etapa previa al rename conserva el
// ejecutable anterior, limpia el temporal propio y no declara éxito.
func TestReplaceSelfUnix_FalloAntesDePublicarConservaAnterior(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el contrato de disponibilidad aplica a Unix")
	}
	for _, failAt := range []string{"copied", "chmod", "sync", "close", "rename"} {
		t.Run(failAt, func(t *testing.T) {
			dir := t.TempDir()
			current, newBin := filepath.Join(dir, "mem"), filepath.Join(dir, "mem-new")
			writeScript037(t, current, "old")
			writeScript037(t, newBin, "new")
			replaceStep = func(stage string) error {
				if stage == failAt {
					return context.Canceled
				}
				return nil
			}
			t.Cleanup(func() { replaceStep = func(string) error { return nil } })

			if err := replaceSelf(current, newBin); err == nil {
				t.Fatal("un fallo antes de publicar no puede declarar éxito")
			}
			if got := runScript037(t, current); got != "old" {
				t.Fatalf("el destino cambió a %q", got)
			}
			assertNoUpdateLeftovers(t, dir, "mem", "mem-new")
		})
	}

	dir := t.TempDir()
	current := filepath.Join(dir, "mem")
	writeScript037(t, current, "old")
	if err := replaceSelf(current, filepath.Join(dir, "no-existe")); err == nil {
		t.Fatal("fuente ausente debe fallar")
	}
	if got := runScript037(t, current); got != "old" {
		t.Fatalf("fuente ausente alteró el destino: %q", got)
	}
	assertNoUpdateLeftovers(t, dir, "mem")
}

// assertNoUpdateLeftovers comprueba que en dir solo quedan los archivos
// esperados: ni temporales ni el respaldo .old de la versión anterior.
func assertNoUpdateLeftovers(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, w := range want {
		allowed[w] = true
	}
	for _, e := range entries {
		if !allowed[e.Name()] {
			t.Fatalf("archivo residual tras la actualización: %s", e.Name())
		}
	}
}
