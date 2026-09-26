// Package release consulta las releases publicadas de gomemory en GitHub
// (feature 034): la última versión, para el aviso de versión nueva, y el
// checksums.txt de cada release, para verificar lo que descarga `mem update`.
package release

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mem/application/ports"
)

// ErrChecksumUnavailable: la release no publica checksums.txt. `mem update`
// aborta en ese caso (FR-032): sin checksum no hay verificación posible.
var ErrChecksumUnavailable = errors.New("la release no publica checksums.txt")

// GitHub implementa ports.ReleasePort sobre la API y las descargas de GitHub.
type GitHub struct {
	apiBase, downloadBase, repo string
	client                      *http.Client
}

var _ ports.ReleasePort = (*GitHub)(nil)

// New crea el adaptador. timeout acota cada llamada (constitución §8).
func New(apiBase, downloadBase, repo string, timeout time.Duration) *GitHub {
	return &GitHub{
		apiBase:      strings.TrimRight(apiBase, "/"),
		downloadBase: strings.TrimRight(downloadBase, "/"),
		repo:         repo,
		client:       &http.Client{Timeout: timeout},
	}
}

func (g *GitHub) Latest(ctx context.Context, etag string) (string, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiBase+"/repos/"+g.repo+"/releases/latest", nil)
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return "", etag, true, nil
	case http.StatusOK:
	default:
		return "", "", false, fmt.Errorf("API de releases: HTTP %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", false, fmt.Errorf("respuesta de la API: %w", err)
	}
	if body.TagName == "" {
		return "", "", false, errors.New("la API no devolvió tag_name")
	}
	return body.TagName, resp.Header.Get("ETag"), false, nil
}

func (g *GitHub) Checksum(ctx context.Context, tag, asset string) (string, error) {
	url := fmt.Sprintf("%s/download/%s/checksums.txt", g.downloadBase, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", ErrChecksumUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("descargar checksums.txt: HTTP %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		// Formato de goreleaser: "<sha256>  <archivo>".
		f := strings.Fields(sc.Text())
		if len(f) == 2 && f[1] == asset {
			return strings.ToLower(f[0]), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksums.txt no incluye %s", asset)
}
