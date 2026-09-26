package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// releaseServer simula la API y las descargas de releases de GitHub.
type releaseServer struct {
	api, download *httptest.Server
	downloads     atomic.Int32
	newBinary     []byte
	asset         string
}

func releaseAssetName() string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("mem_windows_%s.zip", runtime.GOARCH)
	}
	return fmt.Sprintf("mem_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
}

func buildReleaseArchive(t *testing.T, bin []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		w, err := zw.Create("mem.exe")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(bin)
		_ = zw.Close()
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "mem", Mode: 0o755, Size: int64(len(bin))})
	_, _ = tw.Write(bin)
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

// newReleaseServer publica tag con un binario nuevo; checksum = "" usa el
// hash correcto, "-" omite checksums.txt y cualquier otro valor lo publica tal cual.
func newReleaseServer(t *testing.T, tag string, checksum string) *releaseServer {
	t.Helper()
	rs := &releaseServer{newBinary: []byte("#!/bin/sh\necho 'gomemory " + strings.TrimPrefix(tag, "v") + "'\n"), asset: releaseAssetName()}
	archive := buildReleaseArchive(t, rs.newBinary)
	sum := sha256.Sum256(archive)
	if checksum == "" {
		checksum = hex.EncodeToString(sum[:])
	}

	rs.api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
	}))
	rs.download = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			if checksum == "-" {
				http.NotFound(w, r)
				return
			}
			if _, err := fmt.Fprintf(w, "%s  %s\n", checksum, rs.asset); err != nil {
				t.Errorf("escribir checksums: %v", err)
			}
		case strings.HasSuffix(r.URL.Path, "/"+rs.asset):
			rs.downloads.Add(1)
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(func() { rs.api.Close(); rs.download.Close() })
	return rs
}

func (rs *releaseServer) apply(s *lifecycleSandbox) {
	s.Setenv("GOMEMORY_RELEASE_API_BASE", rs.api.URL)
	s.Setenv("GOMEMORY_RELEASE_DOWNLOAD_BASE", rs.download.URL)
}

// FR-006: `./mem update` desde una copia local actualiza el global y retira
// la copia.
func TestUpdate_DesdeCopiaLocalActualizaElGlobal(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	copia := filepath.Join(p, exeName("mem"))
	copyExecutable(t, buildLifecycleBinary(t), copia)
	rs := newReleaseServer(t, "v9.9.9", "")
	rs.apply(s)

	res := s.Run(copia, p, "update", "--version", "v9.9.9")
	if res.ExitCode != 0 {
		t.Fatalf("update: code=%d\n%s\n%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	got, err := os.ReadFile(s.GlobalBin)
	if err != nil || !bytes.Equal(got, rs.newBinary) {
		t.Fatalf("el global debía quedar con el binario nuevo (err=%v)", err)
	}
	if _, err := os.Stat(copia); !os.IsNotExist(err) {
		t.Error("la copia local debía retirarse tras actualizar el global")
	}
}

// R6: con el global sin escritura no se descarga nada y se indica el comando.
func TestUpdate_GlobalSinEscrituraNoDescarga(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permisos de directorio de Unix")
	}
	s := newLifecycleSandbox(t)
	p := s.Project("p2")
	copia := filepath.Join(p, exeName("mem"))
	copyExecutable(t, buildLifecycleBinary(t), copia)
	rs := newReleaseServer(t, "v9.9.9", "")
	rs.apply(s)
	if err := os.Chmod(s.BinDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.BinDir, 0o755) })

	res := s.Run(copia, p, "update", "--version", "v9.9.9")
	if res.ExitCode != 1 {
		t.Fatalf("debe terminar con 1: code=%d\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	if rs.downloads.Load() != 0 {
		t.Error("no debía descargarse nada")
	}
	if out := res.Stdout + res.Stderr; !strings.Contains(out, "sudo mem update") {
		t.Errorf("debe indicar el comando exacto:\n%s", out)
	}
}

// updateSummary devuelve el bloque final "Resumen:" de update. El refresco de
// la integración lanza install, que imprime su propio resumen antes.
func updateSummary(t *testing.T, stdout string) string {
	t.Helper()
	i := strings.LastIndex(stdout, "Resumen:")
	if i < 0 {
		t.Fatalf("falta el resumen final por pasos:\n%s", stdout)
	}
	return stdout[i:]
}

// FR-024: update termina con un resumen de ✓/⚠/✗ por paso, también cuando un
// paso aborta la operación.
func TestUpdate_TerminaConResumenDePasos(t *testing.T) {
	t.Run("éxito desde una copia local", func(t *testing.T) {
		s := newLifecycleSandbox(t)
		p := s.Project("p1")
		copia := filepath.Join(p, exeName("mem"))
		copyExecutable(t, buildLifecycleBinary(t), copia)
		newReleaseServer(t, "v9.9.9", "").apply(s)

		res := s.Run(copia, p, "update", "--version", "v9.9.9")
		if res.ExitCode != 0 {
			t.Fatalf("update: code=%d\n%s\n%s", res.ExitCode, res.Stdout, res.Stderr)
		}
		resumen := updateSummary(t, res.Stdout)
		for _, paso := range []string{"✓ Descarga", "✓ Checksum", "✓ Binario", "✓ Copia local", "✓ Integración del proyecto"} {
			if !strings.Contains(resumen, paso) {
				t.Errorf("falta %q en el resumen:\n%s", paso, resumen)
			}
		}
	})
	t.Run("checksum inválido", func(t *testing.T) {
		s := newLifecycleSandbox(t)
		newReleaseServer(t, "v9.9.9", strings.Repeat("0", 64)).apply(s)

		res := s.Run("", s.Root, "update", "--version", "v9.9.9")
		if res.ExitCode != 1 {
			t.Fatalf("code=%d; quiero 1\n%s", res.ExitCode, res.Stdout+res.Stderr)
		}
		resumen := updateSummary(t, res.Stdout)
		if !strings.Contains(resumen, "✓ Descarga") || !strings.Contains(resumen, "✗ Checksum") {
			t.Errorf("el resumen debe marcar el checksum como fallido:\n%s", resumen)
		}
		if strings.Contains(resumen, "Binario") {
			t.Errorf("no debe listar pasos que no se ejecutaron:\n%s", resumen)
		}
	})
	t.Run("global sin escritura", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("permisos de directorio de Unix")
		}
		s := newLifecycleSandbox(t)
		p := s.Project("p2")
		copia := filepath.Join(p, exeName("mem"))
		copyExecutable(t, buildLifecycleBinary(t), copia)
		newReleaseServer(t, "v9.9.9", "").apply(s)
		if err := os.Chmod(s.BinDir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(s.BinDir, 0o755) })

		res := s.Run(copia, p, "update", "--version", "v9.9.9")
		if res.ExitCode != 1 {
			t.Fatalf("code=%d; quiero 1\n%s", res.ExitCode, res.Stdout+res.Stderr)
		}
		resumen := updateSummary(t, res.Stdout)
		if !strings.Contains(resumen, "✗ Binario") || !strings.Contains(resumen, "sudo mem update") {
			t.Errorf("el resumen debe marcar el binario y dar el comando manual:\n%s", resumen)
		}
	})
}

// FR-032, SC-009: un checksum alterado o ausente aborta sin tocar el binario.
func TestUpdate_ChecksumInvalidoAborta(t *testing.T) {
	for name, sum := range map[string]string{"alterado": strings.Repeat("0", 64), "ausente": "-"} {
		t.Run(name, func(t *testing.T) {
			s := newLifecycleSandbox(t)
			rs := newReleaseServer(t, "v9.9.9", sum)
			rs.apply(s)
			antes := readAll(t, s.GlobalBin)

			res := s.Run("", s.Root, "update", "--version", "v9.9.9")
			if res.ExitCode != 1 {
				t.Fatalf("code=%d; quiero 1\n%s", res.ExitCode, res.Stdout+res.Stderr)
			}
			if readAll(t, s.GlobalBin) != antes {
				t.Fatal("el binario instalado cambió pese al checksum inválido")
			}
			if out := res.Stdout + res.Stderr; !strings.Contains(out, "checksum") {
				t.Errorf("debe explicar el motivo:\n%s", out)
			}
		})
	}
}

// FR-033: doctor informa de la versión global, las copias locales y la caché.
func TestDoctor_VersionYBinario(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	// Una copia antigua en el proyecto, sin retirar todavía (no pasó por
	// install ni session-start).
	copia := filepath.Join(p, exeName("mem"))
	copyExecutable(t, buildLifecycleBinary(t), copia)
	mustWrite(t, filepath.Join(s.Data, "update-check.json"),
		`{"latest":"v99.0.0","checked_at":"2026-09-26T10:00:00Z","etag":"","last_error":""}`)

	res := s.Run("", p, "doctor", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("doctor: %d\n%s", res.ExitCode, res.Stderr)
	}
	var got struct {
		Binary struct {
			GlobalPath    string `json:"global_path"`
			GlobalVersion string `json:"global_version"`
			LocalCopies   []struct {
				Path, Version string
			} `json:"local_copies"`
			UpdateCheck struct {
				Enabled   bool   `json:"enabled"`
				Latest    string `json:"latest"`
				CheckedAt string `json:"checked_at"`
			} `json:"update_check"`
		} `json:"binary"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &got); err != nil {
		t.Fatalf("JSON: %v\n%s", err, res.Stdout)
	}
	v := got.Binary
	if v.GlobalPath != s.GlobalBin || v.GlobalVersion == "" {
		t.Errorf("global = %q %q", v.GlobalPath, v.GlobalVersion)
	}
	if len(v.LocalCopies) != 1 || !strings.HasSuffix(v.LocalCopies[0].Path, exeName("mem")) {
		t.Errorf("copias locales = %+v", v.LocalCopies)
	}
	// El sandbox desactiva la consulta con GOMEMORY_NO_UPDATE_CHECK.
	if v.UpdateCheck.Enabled || v.UpdateCheck.Latest != "v99.0.0" {
		t.Errorf("update_check = %+v", v.UpdateCheck)
	}

	text := s.Run("", p, "doctor")
	if !strings.Contains(text.Stdout, "Versión y binario") || !strings.Contains(text.Stdout, "GOMEMORY_NO_UPDATE_CHECK") {
		t.Errorf("sección de texto:\n%s", text.Stdout)
	}
}
