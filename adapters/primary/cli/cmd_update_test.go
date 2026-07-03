package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.9.0"})
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
		w.Write([]byte("not found"))
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
	tw.Write(content)
	tw.Close()
	gz.Close()
	os.WriteFile(archivePath, buf.Bytes(), 0644)

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
	fw.Write(content)
	zw.Close()
	os.WriteFile(archivePath, buf.Bytes(), 0644)

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
	tw.Close()
	gz.Close()
	os.WriteFile(archivePath, buf.Bytes(), 0644)

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

	os.WriteFile(current, []byte("old content"), 0755)
	os.WriteFile(newBin, []byte("new content"), 0755)

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
