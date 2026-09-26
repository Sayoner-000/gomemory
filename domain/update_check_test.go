package domain

import (
	"errors"
	"testing"
	"time"
)

func TestUpdateCheck_CaducidadYTransiciones(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	base := UpdateCheck{Latest: "v2.26.4", ETag: "etag-1", CheckedAt: now.Add(-UpdateCheckTTL)}
	if base.Stale(now) {
		t.Fatal("la caché no vence exactamente en el límite")
	}
	if !base.Stale(now.Add(time.Nanosecond)) {
		t.Fatal("la caché debe vencer después del límite")
	}

	failed := base.RecordFailure(now, errors.New("sin red"))
	if failed.Latest != base.Latest || failed.ETag != base.ETag || failed.LastError != "sin red" || !failed.CheckedAt.Equal(now) {
		t.Fatalf("un fallo debe conservar la versión y el ETag: %+v", failed)
	}
	unchanged := failed.RecordSuccess(now.Add(time.Hour), "", "")
	if unchanged.Latest != base.Latest || unchanged.ETag != base.ETag || unchanged.LastError != "" {
		t.Fatalf("304 debe conservar lo conocido y borrar el error: %+v", unchanged)
	}
	updated := unchanged.RecordSuccess(now.Add(2*time.Hour), "v2.27.0", "etag-2")
	if updated.Latest != "v2.27.0" || updated.ETag != "etag-2" || !updated.CheckedAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("200 debe actualizar versión, ETag y fecha: %+v", updated)
	}
}
