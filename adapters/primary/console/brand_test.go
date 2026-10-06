package console

import (
	"bytes"
	"strings"
	"testing"
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
			if strings.Contains(out.String(), "goMemory") != tc.wantLogo || strings.Contains(out.String(), "\x1b[") != tc.wantANSI {
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
