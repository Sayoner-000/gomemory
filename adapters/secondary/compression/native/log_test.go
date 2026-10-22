package native

import (
	"fmt"
	"strings"
	"testing"
)

// T020 — logs: ERROR/FATAL/PANIC y cabecera de traza intactos, series colapsadas.
func TestCompressLogCorpus(t *testing.T) {
	for _, name := range []string{"gotest-fail.log", "py-traceback.log"} {
		in := readCorpus(t, name)
		out, n := compressLog(in, "abc123abc123", maxTh)
		if n == 0 {
			t.Fatalf("%s: sin compresión", name)
		}
		if err := checkLiteral(in, out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, l := range strings.Split(in, "\n") {
			if protectedLine.MatchString(l) && !strings.Contains(out, l) {
				t.Errorf("%s: línea protegida omitida: %q", name, l)
			}
		}
	}
}

func TestCompressLogRunAndTrace(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 312; i++ {
		fmt.Fprintf(&b, "2026-09-25 10:00:%02d INFO petición %d servida en %dms\n", i%60, i, 10+i%7)
	}
	b.WriteString("goroutine 1 [running]:\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "main.f%d(0x1)\n\t/src/main.go:%d +0x1\n", i, i)
	}
	in := b.String()
	out, _ := compressLog(in, "r", maxTh)
	if !strings.Contains(out, "×310 líneas similares") {
		t.Errorf("la serie de 312 debe quedar en primera + última + ×310:\n%s", out)
	}
	if !strings.Contains(out, "goroutine 1 [running]:") || !strings.Contains(out, "main.f0(0x1)") || !strings.Contains(out, "main.f4(0x1)") {
		t.Error("cabecera y primeros marcos de la traza deben conservarse")
	}
	if !strings.Contains(out, "líneas de traza") {
		t.Error("los marcos restantes deben omitirse con marcador")
	}
}

func TestCompressDiffAndTable(t *testing.T) {
	diff := readCorpus(t, "repo.diff")
	out, _ := compressDiff(diff, "r", maxTh)
	if err := checkLiteral(diff, out); err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(diff, "\n") {
		if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-") || strings.HasPrefix(l, "@@")) && !strings.Contains(out, l) {
			t.Errorf("diff: línea cambiada o cabecera omitida: %q", l)
		}
	}
	table := readCorpus(t, "listing.txt")
	tout, n := compressTable(table, "r", maxTh)
	lines := strings.Split(table, "\n")
	if n == 0 || !strings.HasPrefix(tout, lines[0]+"\n"+lines[1]) {
		t.Errorf("tabla: debe conservar cabecera y separador (n=%d)", n)
	}
	if err := checkLiteral(table, tout); err != nil {
		t.Error(err)
	}
}

// En un log con prefijo (`gh run view --log`: job, paso y hora separados por
// tabuladores) el router ve una tabla; los fallos de prueba no empiezan la
// línea y aun así deben conservarse.
func TestCompressTable_ConservaFallosConPrefijo(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "Linux\tgo test\t2026-10-22T12:00:%02dZ ok  \tmem/pkg%d\t0.1s\n", i, i)
		if i == 20 {
			b.WriteString("Linux\tgo test\t2026-10-22T12:00:20Z --- FAIL: TestX (0.01s)\n")
			b.WriteString("Linux\tgo test\t2026-10-22T12:00:20Z FAIL\tmem/pkg20\t0.2s\n")
		}
	}
	out, n := compressTable(b.String(), "r", maxTh)
	if n == 0 {
		t.Fatal("las filas repetidas deben omitirse")
	}
	for _, want := range []string{"--- FAIL: TestX", "FAIL\tmem/pkg20"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en la salida:\n%s", want, out)
		}
	}
}
