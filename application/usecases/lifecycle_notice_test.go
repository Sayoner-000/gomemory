package usecases

import (
	"errors"
	"testing"
	"time"

	"mem/domain"
)

var t0 = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func baseInput() UpdateNoticeInput {
	return UpdateNoticeInput{
		Current:   "2.26.4",
		Cache:     domain.UpdateCheck{Latest: "v2.27.0", CheckedAt: t0.Add(-time.Hour)},
		CacheOK:   true,
		Now:       t0,
		SessionID: "s1",
	}
}

// FR-030: versión publicada mayor → aviso con el comando exacto.
func TestDecideUpdateNotice_Aviso(t *testing.T) {
	d := DecideUpdateNotice(baseInput())
	want := "gomemory v2.27.0 disponible (tienes v2.26.4) → mem update"
	if d.Notice != want {
		t.Fatalf("aviso = %q; quiero %q", d.Notice, want)
	}
	if d.Mark == nil || d.Mark.Version != "v2.27.0" || d.Mark.Session != "s1" {
		t.Errorf("marca = %+v", d.Mark)
	}
	if d.RefreshNeeded {
		t.Error("una caché de hace 1 h no está vencida")
	}
}

// FR-030: como mucho una vez por versión y sesión.
func TestDecideUpdateNotice_UnaVezPorVersionYSesion(t *testing.T) {
	in := baseInput()
	in.LastNotice = domain.UpdateNotice{Version: "v2.27.0", Session: "s1"}
	if d := DecideUpdateNotice(in); d.Notice != "" {
		t.Errorf("misma versión y sesión: no se repite (%q)", d.Notice)
	}
	in.SessionID = "s2"
	if d := DecideUpdateNotice(in); d.Notice == "" {
		t.Error("en otra sesión se vuelve a avisar")
	}
	in.SessionID = "s1"
	in.Cache.Latest = "v2.27.1"
	if d := DecideUpdateNotice(in); d.Notice == "" {
		t.Error("una versión más nueva vuelve a avisar en la misma sesión")
	}
}

func TestDecideUpdateNotice_SinAviso(t *testing.T) {
	cases := map[string]func(*UpdateNoticeInput){
		"al día":                func(in *UpdateNoticeInput) { in.Cache.Latest = "v2.26.4" },
		"instalada más nueva":   func(in *UpdateNoticeInput) { in.Current = "2.28.0" },
		"prerrelease":           func(in *UpdateNoticeInput) { in.Cache.Latest = "v2.27.0-rc.1" },
		"sin caché":             func(in *UpdateNoticeInput) { in.CacheOK = false },
		"versión de desarrollo": func(in *UpdateNoticeInput) { in.Current = "dev" },
	}
	for name, mod := range cases {
		in := baseInput()
		mod(&in)
		if d := DecideUpdateNotice(in); d.Notice != "" {
			t.Errorf("%s: no debe avisar (%q)", name, d.Notice)
		}
	}
}

// FR-028: se refresca si la caché falta o supera las 24 h.
func TestDecideUpdateNotice_Refresco(t *testing.T) {
	in := baseInput()
	in.Cache.CheckedAt = t0.Add(-25 * time.Hour)
	if !DecideUpdateNotice(in).RefreshNeeded {
		t.Error("más de 24 h: hay que refrescar")
	}
	in = baseInput()
	in.CacheOK = false
	if !DecideUpdateNotice(in).RefreshNeeded {
		t.Error("sin caché: hay que refrescar")
	}
}

// FR-031, SC-008: desactivado no hay ni aviso ni consulta.
func TestDecideUpdateNotice_Desactivado(t *testing.T) {
	in := baseInput()
	in.Disabled = true
	in.CacheOK = false
	d := DecideUpdateNotice(in)
	if d.Notice != "" || d.RefreshNeeded || d.Mark != nil {
		t.Fatalf("desactivado = %+v", d)
	}
}

// R4: un fallo de consulta conserva latest y etag y solo actualiza el momento y
// el error, para reintentar pasado el intervalo y no en cada sesión.
func TestUpdateCheck_RecordFailureConserva(t *testing.T) {
	c := domain.UpdateCheck{Latest: "v2.27.0", ETag: `"e"`, CheckedAt: t0.Add(-48 * time.Hour)}
	got := c.RecordFailure(t0, errors.New("sin red"))
	if got.Latest != "v2.27.0" || got.ETag != `"e"` || !got.CheckedAt.Equal(t0) || got.LastError != "sin red" {
		t.Fatalf("RecordFailure = %+v", got)
	}
	ok := got.RecordSuccess(t0.Add(time.Hour), "v2.28.0", `"f"`)
	if ok.Latest != "v2.28.0" || ok.ETag != `"f"` || ok.LastError != "" {
		t.Fatalf("RecordSuccess = %+v", ok)
	}
	nm := ok.RecordSuccess(t0.Add(2*time.Hour), "", `"f"`) // 304: sin tag nuevo
	if nm.Latest != "v2.28.0" {
		t.Errorf("un 304 conserva latest: %+v", nm)
	}
}
