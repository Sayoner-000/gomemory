package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// memStore es un OriginalStoreRepository en memoria para las pruebas del motor.
type memStore struct {
	mu      sync.Mutex
	byRef   map[string]string
	puts    int
	failPut error
	delay   time.Duration
}

func newMemStore() *memStore { return &memStore{byRef: map[string]string{}} }

func (m *memStore) Put(ctx context.Context, _, content, _, _ string) (string, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if m.failPut != nil {
		return "", m.failPut
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	sum := sha256.Sum256([]byte(content))
	ref := domain.RefFromHash(hex.EncodeToString(sum[:]), domain.RefLen)
	m.byRef[ref] = content
	return ref, nil
}

func (m *memStore) Get(_ context.Context, ref string) (string, ports.OriginalMeta, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.byRef[ref]
	return c, ports.OriginalMeta{Ref: ref}, ok, nil
}

func (m *memStore) Purge(context.Context) (int, int64, error) { return 0, 0, nil }
func (m *memStore) Usage(context.Context) (int64, int, error) { return 0, len(m.byRef), nil }

// memStats cuenta las llamadas a Record.
type memStats struct {
	mu      sync.Mutex
	records []ports.CompressionResult
	fail    bool
}

func (s *memStats) Record(_ context.Context, _ string, r ports.CompressionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	if s.fail {
		return errors.New("fallo de estadísticas")
	}
	return nil
}
func (s *memStats) RecordRetrieval(context.Context, string, string, string) error { return nil }
func (s *memStats) Summary(context.Context, string) ([]ports.CompressorStats, error) {
	return nil, nil
}
