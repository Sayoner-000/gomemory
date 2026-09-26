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

func newServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// R4: la consulta usa ETag; un 304 no descarga nada y conserva la versión.
func TestLatest_ConETag(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	g := New(srv.URL, srv.URL, repo, time.Second)

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
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	if _, _, _, err := New(srv.URL, srv.URL, repo, time.Second).Latest(context.Background(), ""); err == nil {
		t.Fatal("un 403 (límite de peticiones) debe ser error")
	}
}

// Constitución §8: toda llamada externa tiene tiempo límite.
func TestLatest_TiempoLimite(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	})
	start := time.Now()
	_, _, _, err := New(srv.URL, srv.URL, repo, 200*time.Millisecond).Latest(context.Background(), "")
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
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/v2.27.0/checksums.txt":
			if _, err := fmt.Fprint(w, sums); err != nil {
				t.Errorf("escribir checksums: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	})
	g := New(srv.URL, srv.URL, repo, time.Second)

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
