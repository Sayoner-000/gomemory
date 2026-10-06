package console

import (
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
			stop := startProgress(&out, e, "Indexando CodeGraph")
			stop()
			stop()
			if tc.wantANSI {
				if !strings.Contains(out.String(), "Indexando CodeGraph") || !strings.HasSuffix(out.String(), "\r\x1b[2K") {
					t.Fatalf("progreso no limpiado = %q", out.String())
				}
			} else if out.Len() != 0 {
				t.Fatalf("animación fuera de terminal compatible: %q", out.String())
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
	stop := startProgress(&out, e, "progreso")
	stop()
	if out.Len() != 0 {
		t.Fatalf("movimiento pese a GOMEMORY_NO_MOTION: %q", out.String())
	}
}
