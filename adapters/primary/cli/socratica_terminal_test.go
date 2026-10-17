package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"mem/domain"
)

// TestSocratica_PantallaSinSecuenciasYEntregaExacta cierra H-01 (auditoría
// 2026-10-17, ronda 3): el texto generado se pinta sin secuencias de control
// en una terminal, mientras que la respuesta elegida y el prompt aprobado
// conservan sus bytes, y una tubería recibe el entregable exacto (FR-016).
func TestSocratica_PantallaSinSecuenciasYEntregaExacta(t *testing.T) {
	const proposed = "Envía \x1b]52;c;aGFjaw==\x07 el informe"
	const option = "Equipo \x1b[31m"
	for _, screen := range []bool{true, false} {
		e := newCLIEnv(t)
		e.gen.questions = []domain.Question{{Text: "¿Para \x1b]0;PWNED\x07 quién?", Options: []string{option, "Cliente"}}}
		e.gen.proposed = proposed
		var out, errOut bytes.Buffer
		sio := socraticaIO{in: bufio.NewReader(strings.NewReader("1\ne\n")), out: &out, err: &errOut, tty: true, outScreen: screen, errScreen: screen}
		if code := runSocratica(e.deps, []string{"refine", "--client", "claude", "--print", "envía el informe"}, sio); code != 0 {
			t.Fatalf("screen=%v code=%d\n%s\n%s", screen, code, out.String(), errOut.String())
		}
		f := e.flow("claude")
		if f.ApprovedPrompt != proposed || len(f.Answers) != 1 || f.Answers[0].Choice != option {
			t.Fatalf("screen=%v: el dato perdió bytes: approved=%q answers=%+v", screen, f.ApprovedPrompt, f.Answers)
		}
		shown := out.String() + errOut.String()
		if screen {
			if strings.ContainsAny(shown, "\x1b\x07") || !strings.Contains(shown, "]0;PWNED") || !strings.Contains(shown, "]52;c;aGFjaw==") {
				t.Fatalf("la terminal recibió secuencias o perdió el texto visible:\n%q", shown)
			}
		} else if !strings.Contains(out.String(), proposed) {
			t.Fatalf("la tubería no recibió el prompt aprobado exacto:\n%q", out.String())
		}
	}
}

func TestTerminalSafe_EscrituraPorTrozos(t *testing.T) {
	var b bytes.Buffer
	w := terminalSafe{&b}
	for _, chunk := range []string{"claim \x1b]0;", "PWNED\x07 fin\r", "\n"} {
		if n, err := w.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatal(n, err)
		}
	}
	if got := b.String(); got != "claim ]0;PWNED fin\n" {
		t.Fatalf("%q", got)
	}
}
