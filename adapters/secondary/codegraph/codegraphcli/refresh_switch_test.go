package codegraphcli

import "testing"

// GOMEMORY_NO_CODE_REFRESH apaga el refresco en segundo plano: con él,
// MaybeRefresh sobre un snapshot vencido ni siquiera llega al debounce, así que
// no lanza el hijo que en Windows bloqueaba la limpieza de la prueba.
func TestMaybeRefresh_DisabledByEnv(t *testing.T) {
	for value, want := range map[string]bool{"": false, "0": false, "false": false, "1": true, "true": true} {
		t.Setenv("GOMEMORY_NO_CODE_REFRESH", value)
		if got := refreshDisabled(); got != want {
			t.Errorf("GOMEMORY_NO_CODE_REFRESH=%q: refreshDisabled() = %v, want %v", value, got, want)
		}
	}
	t.Setenv("GOMEMORY_NO_CODE_REFRESH", "1")
	p := New(t.TempDir(), t.TempDir(), "")
	p.MaybeRefresh()
	if !p.lastTrigger.IsZero() {
		t.Fatal("MaybeRefresh launched a refresh with GOMEMORY_NO_CODE_REFRESH=1")
	}
}
