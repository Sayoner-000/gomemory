package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func copyFileForTest(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// updateTestEnv es el entorno de `mem update` en estas pruebas, con el PATH
// reducido al sistema. Desde la feature 034 (FR-006), update sustituye el `mem`
// global del PATH cuando existe: con el PATH real de quien ejecuta la suite,
// estas pruebas reemplazarían su binario instalado por el binario falso.
func updateTestEnv(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	return append(append(env, "PATH=/usr/bin:/bin"), extra...)
}

// buildFakeTarGz produce el tar.gz que sirve el release fake, con el mismo
// layout que scripts/install.sh espera: un único archivo "mem" en la raíz.
func buildFakeTarGz(content []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "mem", Mode: 0755, Size: int64(len(content))})
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

// TestUpdateIntegration monta un fake de la API de GitHub Releases + la
// descarga de assets, corre `mem update` como subproceso contra un binario
// dummy, y verifica que el binario termina reemplazado con el contenido
// "nuevo" servido por el fake.
func TestUpdateIntegration(t *testing.T) {
	bin := buildMemBinary(t)

	newContent := []byte("#!/bin/sh\necho fake-new-mem\n")
	asset := buildFakeTarGz(newContent)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/Sayoner-000/gomemory/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.9.9"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Desde la feature 034 (FR-032) update verifica el SHA-256 del asset con el
	// checksums.txt de la release, como publica goreleaser.
	sum := sha256.Sum256(asset)
	downloadMux := http.NewServeMux()
	downloadMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/checksums.txt") {
			_, _ = fmt.Fprintf(w, "%s  mem_%s_%s.tar.gz\n", hex.EncodeToString(sum[:]), runtime.GOOS, runtime.GOARCH)
			return
		}
		_, _ = w.Write(asset)
	})
	downloadSrv := httptest.NewServer(downloadMux)
	defer downloadSrv.Close()

	// Copiar el binario de test a un dir aislado y ejecutarlo con las bases
	// de API/descarga apuntando a los servidores fake vía variables de entorno
	// que cmd_update.go debe leer como override para tests.
	target := t.TempDir()
	dummyBin := filepath.Join(target, "mem")
	if err := copyFileForTest(bin, dummyBin); err != nil {
		t.Fatalf("copiar binario dummy: %v", err)
	}
	if err := os.Chmod(dummyBin, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dummyBin, "update", "--version", "v9.9.9")
	cmd.Dir = target
	cmd.Env = updateTestEnv(
		"GOMEMORY_RELEASE_API_BASE="+srv.URL,
		"GOMEMORY_RELEASE_DOWNLOAD_BASE="+downloadSrv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem update: %v\n%s", err, out)
	}

	data, err := os.ReadFile(dummyBin)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if !bytes.Equal(data, newContent) {
		t.Errorf("el binario no se reemplazó con el contenido esperado, got: %q", data)
	}
	if _, err := os.Stat(dummyBin + ".old"); err == nil {
		t.Error("el archivo .old debió limpiarse tras un update exitoso")
	}
}

// TestUpdateCheckDoesNotMutate verifica que --check solo consulta y no
// descarga ni reemplaza el binario.
func TestUpdateCheckDoesNotMutate(t *testing.T) {
	bin := buildMemBinary(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/Sayoner-000/gomemory/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.9.9"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	target := t.TempDir()
	dummyBin := filepath.Join(target, "mem")
	if err := copyFileForTest(bin, dummyBin); err != nil {
		t.Fatalf("copiar binario dummy: %v", err)
	}
	if err := os.Chmod(dummyBin, 0755); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(dummyBin)
	if err != nil {
		t.Fatalf("read dummy binary: %v", err)
	}

	cmd := exec.Command(dummyBin, "update", "--check")
	cmd.Dir = target
	cmd.Env = updateTestEnv("GOMEMORY_RELEASE_API_BASE=" + srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem update --check: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("v9.9.9")) {
		t.Errorf("esperaba que --check mostrara la versión disponible, got: %s", out)
	}

	after, err := os.ReadFile(dummyBin)
	if err != nil {
		t.Fatalf("read dummy binary after check: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("--check no debió modificar el binario")
	}
}
