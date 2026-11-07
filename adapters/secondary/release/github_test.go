package release

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const repo = "Sayoner-000/gomemory"

// handlerTransport atiende cada petición con un handler en memoria. Las
// pruebas no necesitan bind en loopback, que algunos sandboxes prohíben (C-002).
type handlerTransport struct{ h http.Handler }

func (tr handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	tr.h.ServeHTTP(rec, r)
	// Como un transporte real: una petición cancelada (el tiempo límite del
	// cliente) no devuelve respuesta.
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

// newGitHub crea el adaptador contra un host ficticio que atiende h en memoria.
func newGitHub(timeout time.Duration, h http.HandlerFunc) *GitHub {
	const base = "http://release.test"
	g := New(base, base, repo, timeout)
	g.client.Transport = handlerTransport{h}
	return g
}

// R4: la consulta usa ETag; un 304 no descarga nada y conserva la versión.
func TestLatest_ConETag(t *testing.T) {
	g := newGitHub(time.Second, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/"+repo+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc"`)
		if _, err := fmt.Fprint(w, `{"tag_name":"v2.27.0"}`); err != nil {
			t.Errorf("escribir release: %v", err)
		}
	})

	tag, etag, notModified, err := g.Latest(context.Background(), "")
	if err != nil || tag != "v2.27.0" || etag != `"abc"` || notModified {
		t.Fatalf("200: tag=%q etag=%q nm=%v err=%v", tag, etag, notModified, err)
	}
	tag, etag, notModified, err = g.Latest(context.Background(), `"abc"`)
	if err != nil || !notModified || tag != "" || etag != `"abc"` {
		t.Fatalf("304: tag=%q etag=%q nm=%v err=%v", tag, etag, notModified, err)
	}
}

func TestLatest_ErrorHTTP(t *testing.T) {
	g := newGitHub(time.Second, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	if _, _, _, err := g.Latest(context.Background(), ""); err == nil {
		t.Fatal("un 403 (límite de peticiones) debe ser error")
	}
}

// Constitución §8: toda llamada externa tiene tiempo límite.
func TestLatest_TiempoLimite(t *testing.T) {
	g := newGitHub(200*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	})
	start := time.Now()
	_, _, _, err := g.Latest(context.Background(), "")
	if err == nil {
		t.Fatal("debía agotar el tiempo")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("tardó %v con un límite de 200 ms", d)
	}
}

// R5: el checksum se lee de checksums.txt del mismo release.
func TestChecksum(t *testing.T) {
	sums := "aaa111  mem_linux_amd64.tar.gz\nbbb222  mem_darwin_arm64.tar.gz\n"
	g := newGitHub(time.Second, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/v2.27.0/checksums.txt":
			if _, err := fmt.Fprint(w, sums); err != nil {
				t.Errorf("escribir checksums: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	})

	got, err := g.Checksum(context.Background(), "v2.27.0", "mem_darwin_arm64.tar.gz")
	if err != nil || got != "bbb222" {
		t.Fatalf("Checksum = %q, %v", got, err)
	}
	if _, err := g.Checksum(context.Background(), "v2.27.0", "mem_windows_amd64.zip"); err == nil ||
		!strings.Contains(err.Error(), "mem_windows_amd64.zip") {
		t.Errorf("un asset sin línea debe ser error que lo nombre: %v", err)
	}
	if _, err := g.Checksum(context.Background(), "v9.9.9", "x"); !errors.Is(err, ErrChecksumUnavailable) {
		t.Errorf("sin checksums.txt: %v", err)
	}
}

// H-01: ni el JSON de la API ni checksums.txt se leen más allá de
// maxMetadataBytes; lo que quede detrás del tope no cuenta.
func TestMetadataReadsAreBounded(t *testing.T) {
	pad := strings.Repeat("x", maxMetadataBytes)
	g := newGitHub(time.Second, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			// Líneas cortas: una sola línea larga ya la corta bufio.Scanner.
			_, _ = fmt.Fprintf(w, "%sccc333  mem_linux_amd64.tar.gz\n", strings.Repeat("0  other.tar.gz\n", maxMetadataBytes/16+1))
			return
		}
		_, _ = fmt.Fprintf(w, `{"pad":"%s","tag_name":"v9.9.9"}`, pad)
	})
	if tag, _, _, err := g.Latest(context.Background(), ""); err == nil {
		t.Errorf("Latest aceptó metadatos por encima del tope: %q", tag)
	}
	if sum, err := g.Checksum(context.Background(), "v1.0.0", "mem_linux_amd64.tar.gz"); err == nil {
		t.Errorf("Checksum leyó más allá del tope: %q", sum)
	}
}
