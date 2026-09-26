package console

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type plainUI struct {
	in  *bufio.Reader
	out io.Writer
}

func (p *plainUI) writef(format string, args ...any) error {
	_, err := fmt.Fprintf(p.out, format, args...)
	return err
}

// NewPlain crea la UI en texto plano: la misma información que la rica, sin
// estilos, para NO_COLOR, TERM=dumb o terminales estrechas (FR-027).
func NewPlain(in io.Reader, out io.Writer) UI {
	return &plainUI{in: bufio.NewReader(in), out: out}
}

// readLine devuelve la línea sin el salto final. Fin de entrada sin nada
// leído equivale a cancelar.
func (p *plainUI) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err != nil && line == "" {
		return "", ErrCanceled
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (p *plainUI) MultiSelect(title string, opts []Option) ([]string, error) {
	if err := p.writef("%s\n", title); err != nil {
		return nil, err
	}
	for i, o := range opts {
		mark := " "
		if o.Checked {
			mark = "x"
		}
		if err := p.writef("  %d) [%s] %s%s\n", i+1, mark, o.Label, hintSuffix(o.Hint)); err != nil {
			return nil, err
		}
	}
	for {
		if err := p.writef("Números separados por coma o espacio (Enter = los marcados): "); err != nil {
			return nil, err
		}
		line, err := p.readLine()
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			var out []string
			for _, o := range opts {
				if o.Checked {
					out = append(out, o.Value)
				}
			}
			return out, nil
		}
		picked, ok := parseIndexes(line, len(opts))
		if !ok {
			if err := p.writef("Opción no válida: usa números del 1 al %d.\n", len(opts)); err != nil {
				return nil, err
			}
			continue
		}
		out := make([]string, 0, len(picked))
		for i := range opts { // orden de las opciones, no el tecleado
			if picked[i] {
				out = append(out, opts[i].Value)
			}
		}
		return out, nil
	}
}

func (p *plainUI) Select(title string, opts []Option) (string, error) {
	if err := p.writef("%s\n", title); err != nil {
		return "", err
	}
	for i, o := range opts {
		rec := ""
		if o.Recommended {
			rec = " (Recomendado)"
		}
		if err := p.writef("  %d) %s%s%s\n", i+1, o.Label, rec, hintSuffix(o.Hint)); err != nil {
			return "", err
		}
	}
	def := recommendedIndex(opts)
	for {
		if err := p.writef("Elige un número (Enter = %d): ", def+1); err != nil {
			return "", err
		}
		line, err := p.readLine()
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return opts[def].Value, nil
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(opts) {
			if err := p.writef("Opción no válida: usa un número del 1 al %d.\n", len(opts)); err != nil {
				return "", err
			}
			continue
		}
		return opts[n-1].Value, nil
	}
}

func (p *plainUI) Confirm(title string, def bool) (bool, error) {
	hint := "[s/N]"
	if def {
		hint = "[S/n]"
	}
	for {
		if err := p.writef("%s %s ", title, hint); err != nil {
			return false, err
		}
		line, err := p.readLine()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "":
			return def, nil
		case "s", "si", "sí", "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		if err := p.writef("Responde s o n.\n"); err != nil {
			return false, err
		}
	}
}

func (p *plainUI) TypedConfirm(title, word string) (bool, error) {
	if err := p.writef("%s\nEscribe %s para confirmar: ", title, word); err != nil {
		return false, err
	}
	line, err := p.readLine()
	if err != nil {
		return false, err
	}
	return line == word, nil
}

func parseIndexes(line string, n int) (map[int]bool, bool) {
	fields := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return nil, false
	}
	picked := make(map[int]bool, len(fields))
	for _, f := range fields {
		i, err := strconv.Atoi(f)
		if err != nil || i < 1 || i > n {
			return nil, false
		}
		picked[i-1] = true
	}
	return picked, true
}

func hintSuffix(h string) string {
	if h == "" {
		return ""
	}
	return " — " + h
}
