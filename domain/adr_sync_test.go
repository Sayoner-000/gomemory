package domain

import "testing"

func TestValidSyncOrigin(t *testing.T) {
	if ValidSyncOrigin("gomemory") != SyncOriginGomemory {
		t.Fatal("gomemory debería validar a SyncOriginGomemory")
	}
	if ValidSyncOrigin("provider") != SyncOriginProvider {
		t.Fatal("provider debería validar a SyncOriginProvider")
	}
	if ValidSyncOrigin("lo-que-sea") != SyncOriginProvider {
		t.Fatal("un origin desconocido debería caer a provider (el más conservador: no asumir propiedad)")
	}
}

func TestValidSyncStatus(t *testing.T) {
	cases := map[string]SyncStatus{
		"ok":                SyncStatusOK,
		"pending":           SyncStatusPending,
		"failed":            SyncStatusFailed,
		"conflict_resolved": SyncStatusConflictResolved,
		"basura":            SyncStatusPending, // default seguro: no asumir éxito sobre un valor desconocido
	}
	for in, want := range cases {
		if got := ValidSyncStatus(in); got != want {
			t.Errorf("ValidSyncStatus(%q) = %q, se esperaba %q", in, got, want)
		}
	}
}
