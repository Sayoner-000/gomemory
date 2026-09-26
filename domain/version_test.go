package domain

import (
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want Version
	}{
		{"v2.26.4", true, Version{Major: 2, Minor: 26, Patch: 4}},
		{"2.26.4", true, Version{Major: 2, Minor: 26, Patch: 4}},
		{" v2.8.0\n", true, Version{Major: 2, Minor: 8}},
		{"v3.0.0-rc.1", true, Version{Major: 3, Pre: "rc.1"}},
		{"gomemory 2.26.4", false, Version{}},
		{"v2.26", false, Version{}},
		{"v2.x.4", false, Version{}},
		{"", false, Version{}},
		{"v-1.0.0", false, Version{}},
	}
	for _, c := range cases {
		got, ok := ParseVersion(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseVersion(%q) = %+v,%v; quiero %+v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNewer(t *testing.T) {
	v := func(s string) Version {
		t.Helper()
		out, ok := ParseVersion(s)
		if !ok {
			t.Fatalf("versión inválida en la prueba: %q", s)
		}
		return out
	}
	cases := []struct {
		a, b string
		want bool
	}{
		{"v2.26.5", "v2.26.4", true},
		{"v2.27.0", "v2.26.9", true},
		{"v3.0.0", "v2.99.99", true},
		{"v2.26.4", "v2.26.4", false},
		{"v2.26.3", "v2.26.4", false},
		{"v2.8.0", "v2.26.4", false}, // comparación numérica, no lexicográfica
		// Una prerrelease nunca es "más nueva" a efectos del aviso (R4).
		{"v2.27.0-rc.1", "v2.26.4", false},
	}
	for _, c := range cases {
		if got := Newer(v(c.a), v(c.b)); got != c.want {
			t.Errorf("Newer(%s, %s) = %v; quiero %v", c.a, c.b, got, c.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	if got := (Version{Major: 2, Minor: 26, Patch: 4}).String(); got != "v2.26.4" {
		t.Errorf("String() = %q", got)
	}
	if got := (Version{Major: 3, Pre: "rc.1"}).String(); got != "v3.0.0-rc.1" {
		t.Errorf("String() con prerrelease = %q", got)
	}
}

func TestUpdateCheckTTL(t *testing.T) {
	if UpdateCheckTTL != 24*time.Hour {
		t.Fatalf("UpdateCheckTTL = %v; FR-028 exige 24 h", UpdateCheckTTL)
	}
}
