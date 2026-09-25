package usecases

import (
	"context"
	"fmt"
	"strings"

	"mem/application/ports"
)

// ErrOriginalNotFound se devuelve cuando una ref no existe o ya caducó.
var ErrOriginalNotFound = fmt.Errorf("ref no encontrada o caducada; vuelve a pedir el contenido a su fuente")

// RetrieveOriginal devuelve el original exacto de una omisión ⟦mem⟧ (FR-013).
func RetrieveOriginal(ctx context.Context, store ports.OriginalStoreRepository, ref string) (string, ports.OriginalMeta, error) {
	ref = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ref), "ref="))
	if store == nil || ref == "" {
		return "", ports.OriginalMeta{}, ErrOriginalNotFound
	}
	content, meta, found, err := store.Get(ctx, ref)
	if err != nil {
		return "", ports.OriginalMeta{}, fmt.Errorf("recuperar %s: %w", ref, err)
	}
	if !found {
		return "", ports.OriginalMeta{}, ErrOriginalNotFound
	}
	return content, meta, nil
}
