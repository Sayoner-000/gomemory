package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func copyFileForTest(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// buildFakeTarGz produce el tar.gz que sirve el release fake, con el mismo
// layout que scripts/install.sh espera: un único archivo "mem" en la raíz.
func buildFakeTarGz(content []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "mem", Mode: 0755, Size: int64(len(content))})
	tw.Write(content)
	tw.Close()
	gz.Close()
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
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.9.9"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	downloadMux := http.NewServeMux()
	downloadMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(asset)
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
	os.Chmod(dummyBin, 0755)

	cmd := exec.Command(dummyBin, "update", "--version", "v9.9.9")
	cmd.Dir = target
	cmd.Env = append(os.Environ(),
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
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.9.9"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	target := t.TempDir()
	dummyBin := filepath.Join(target, "mem")
	if err := copyFileForTest(bin, dummyBin); err != nil {
		t.Fatalf("copiar binario dummy: %v", err)
	}
	os.Chmod(dummyBin, 0755)

	before, err := os.ReadFile(dummyBin)
	if err != nil {
		t.Fatalf("read dummy binary: %v", err)
	}

	cmd := exec.Command(dummyBin, "update", "--check")
	cmd.Dir = target
	cmd.Env = append(os.Environ(), "GOMEMORY_RELEASE_API_BASE="+srv.URL)
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
