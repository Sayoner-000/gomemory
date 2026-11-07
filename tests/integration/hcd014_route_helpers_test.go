package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// This is an isolated wire-format bench, not an installed forwarding proxy.
// Both requests terminate on the loopback server; no credentials are involved.
func hcd014Capture(t *testing.T, body []byte, transform func([]byte) []byte) []byte {
	t.Helper()
	skipIfNoLoopback(t)
	capture := make(chan []byte, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read failed", 400)
			return
		}
		if transform != nil {
			b = transform(b)
		}
		capture <- bytes.Clone(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fixture":true}`))
	}))
	defer s.Close()
	client := s.Client()
	client.Timeout = 3 * time.Second
	resp, err := client.Post(s.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("capture: %v status %d", err, resp.StatusCode)
	}
	select {
	case got := <-capture:
		return got
	default:
		t.Fatal("no request captured")
		return nil
	}
}
