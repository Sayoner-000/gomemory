package console

import (
	"mem/version"

	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestBrand_RespetaTerminalYSalidaRedirigida(t *testing.T) {
	for _, tc := range []struct {
		name               string
		tty                bool
		width              int
		env                map[string]string
		wantLogo, wantANSI bool
	}{
		{"terminal", true, 80, nil, true, true},
		{"pipe", false, 80, nil, false, false},
		{"ci", true, 80, map[string]string{"CI": "true"}, false, false},
		{"sin color", true, 80, map[string]string{"NO_COLOR": "1"}, true, false},
		{"dumb", true, 80, map[string]string{"TERM": "dumb"}, true, false},
		{"estrecha", true, 30, nil, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := Env{StdoutTTY: tc.tty, Width: tc.width, Getenv: func(k string) string { return tc.env[k] }}
			var out bytes.Buffer
			printBrand(&out, e, "index")
			if strings.Contains(ansi.Strip(out.String()), "goMemory") != tc.wantLogo || strings.Contains(out.String(), "\x1b[") != tc.wantANSI {
				t.Fatalf("banner = %q", out.String())
			}
			out.Reset()
			flow := NewFlow(&out, e, "Indexar")
			flow.Progress("Indexando CodeGraph")
			flow.End("")
			flow.Stop()
			if redraws := strings.Contains(out.String(), "\x1b[J"); redraws != animatedTerminal(e) {
				t.Fatalf("redibujo=%v en terminal animada=%v: %q", redraws, animatedTerminal(e), out.String())
			}
			if !tc.wantANSI && strings.Contains(out.String(), "\x1b[") {
				t.Fatalf("secuencias de control fuera de terminal compatible: %q", out.String())
			}
		})
	}
}

func TestBrandVariantsFitAndQueriesStayCompact(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120} {
		for _, action := range []string{"help", "install", "usage", "search"} {
			e := Env{StdoutTTY: true, Width: width, Getenv: func(string) string { return "" }}
			var out bytes.Buffer
			printBrand(&out, e, action)
			for _, line := range strings.Split(out.String(), "\n") {
				if ansi.StringWidth(line) > width {
					t.Errorf("%s en %d: %q", action, width, line)
				}
			}
			if action == "usage" || action == "search" {
				if strings.Count(out.String(), "\n") > 5 {
					t.Errorf("banner excesivo para consulta %s", action)
				}
			}
		}
	}
}

func TestNoMotionPreservesColorButStopsProgress(t *testing.T) {
	e := Env{StdoutTTY: true, Width: 80, Getenv: func(k string) string {
		if k == "GOMEMORY_NO_MOTION" {
			return "1"
		}
		return ""
	}}
	var out bytes.Buffer
	printBrand(&out, e, "help")
	if !strings.Contains(out.String(), "\x1b[") {
		t.Fatal("desactivar movimiento eliminó el tema")
	}
	out.Reset()
	flow := NewFlow(&out, e, "Indexar")
	flow.Progress("progreso")
	flow.End("")
	if strings.Contains(out.String(), "\x1b[J") {
		t.Fatalf("movimiento pese a GOMEMORY_NO_MOTION: %q", out.String())
	}
}

func TestBrandCompactaUsaCabeceraUnificadaConRegla(t *testing.T) {
	var out bytes.Buffer
	printBrand(&out, styledEnv(90), "usage")
	lines := strings.Split(strings.Trim(ansi.Strip(out.String()), "\n"), "\n")
	if len(lines) != 2 || lines[0] != "◆ goMemory "+version.Version+" › usage" {
		t.Fatalf("cabecera = %q", lines)
	}
	if strings.Trim(lines[1], "─") != "" || ansi.StringWidth(lines[1]) != 90 {
		t.Fatalf("regla = %q", lines[1])
	}
}
