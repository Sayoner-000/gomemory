package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// unresolvedOutput es el texto de un printf cuya salida no se puede calcular
// sin ejecutarlo. Si acaba como comando (`$(printf '%q' …) …`, `printf … |
// sh`), destructive-shell lo bloquea en vez de suponer que es inocuo (B-001).
const unresolvedOutput = "\x00salida-no-resoluble"

// Topes de printf: un ancho enorme (`%999999999s`) no debe reservar memoria
// en el hook; más allá, la salida se da por no resoluble.
const (
	printfMaxWidth  = 1024
	printfMaxOutput = 64 << 10
)

// echoText es la salida de echo: las opciones -n, -e y -E solo cuentan al
// principio. Los escapes se interpretan salvo con -E, como hacen dash y zsh
// por defecto; bash sin -e los deja tal cual, pero ante la duda se toma la
// lectura que puede ejecutar algo (`$(echo 'r\x6d') -rf x`).
func echoText(args []string) string {
	escapes := true
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' && strings.Trim(args[0][1:], "neE") == "" {
		if i := strings.LastIndexAny(args[0], "eE"); i > 0 {
			escapes = args[0][i] == 'e'
		}
		args = args[1:]
	}
	text := strings.Join(args, " ")
	if escapes {
		text, _ = decodeEchoEscapes(text)
	}
	return text
}

// decodeEchoEscapes decodifica los escapes de echo y de `%b`: los de
// `$'…'`, con el octal escrito `\0NNN`, y `\c`, que corta la salida (stop).
func decodeEchoEscapes(s string) (string, bool) {
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' || i+1 >= len(runes) {
			b.WriteRune(runes[i])
			continue
		}
		switch runes[i+1] {
		case 'c':
			return b.String(), true
		case '0':
			v, n := 0, 0
			for n < 3 && i+2+n < len(runes) && runes[i+2+n] >= '0' && runes[i+2+n] <= '7' {
				v = v*8 + int(runes[i+2+n]-'0')
				n++
			}
			b.WriteRune(rune(v))
			i += 1 + n
		default:
			dec, j := decodeANSIEscape(runes, i+1)
			b.WriteString(dec)
			i = j
		}
	}
	return b.String(), false
}

// printfOutput calcula la salida de printf sin ejecutarlo: los escapes del
// formato y las conversiones %s, %b, %c, %d, %i, %u, %o, %x, %X, %e, %f,
// %g y %% con sus opciones, ancho y precisión (también `*`). Como el shell,
// reutiliza el formato mientras queden argumentos. Una conversión que no se
// sabe calcular (`%q`) o un número inválido dan ok=false.
func printfOutput(args []string) (string, bool) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) > 0 && strings.HasPrefix(args[0], "-v") {
		return "", true // `printf -v var …` asigna, no escribe.
	}
	if len(args) == 0 {
		return "", false
	}
	format, args := []rune(args[0]), args[1:]
	var b strings.Builder
	for {
		used := 0
		next := func() string {
			if len(args) == 0 {
				return ""
			}
			a := args[0]
			args = args[1:]
			used++
			return a
		}
		for i := 0; i < len(format); i++ {
			r := format[i]
			switch {
			case r == '\\' && i+1 < len(format):
				var dec string
				dec, i = decodeANSIEscape(format, i+1)
				b.WriteString(dec)
			case r == '%':
				if i+1 >= len(format) {
					return "", false // bash: «missing format character».
				}
				var text string
				var ok, stop bool
				text, i, ok, stop = printfDirective(format, i+1, next)
				if !ok {
					return "", false
				}
				b.WriteString(text)
				if stop {
					return b.String(), true
				}
			default:
				b.WriteRune(r)
			}
			if b.Len() > printfMaxOutput {
				return "", false
			}
		}
		if len(args) == 0 || used == 0 {
			return b.String(), true
		}
	}
}

// printfDirective resuelve la conversión que empieza en j (tras el `%`) y
// devuelve su texto y la posición de su último carácter. stop indica un
// `\c` dentro de %b, que corta toda la salida.
func printfDirective(format []rune, j int, next func() string) (text string, end int, ok, stop bool) {
	spec := "%"
	for j < len(format) && strings.ContainsRune("-+ #0", format[j]) {
		spec += string(format[j])
		j++
	}
	number := func() bool {
		if j < len(format) && format[j] == '*' {
			n, err := strconv.Atoi(next())
			if err != nil || n > printfMaxWidth || n < -printfMaxWidth {
				return false
			}
			spec += strconv.Itoa(n)
			j++
			return true
		}
		start := j
		for j < len(format) && format[j] >= '0' && format[j] <= '9' {
			j++
		}
		if n, _ := strconv.Atoi(string(format[start:j])); n > printfMaxWidth {
			return false
		}
		spec += string(format[start:j])
		return true
	}
	if !number() {
		return "", j, false, false
	}
	if j < len(format) && format[j] == '.' {
		spec += "."
		j++
		if !number() {
			return "", j, false, false
		}
	}
	if j >= len(format) {
		return "", j, false, false
	}
	switch conv := format[j]; conv {
	case '%':
		return "%", j, true, false
	case 's':
		return fmt.Sprintf(spec+"s", next()), j, true, false
	case 'b':
		dec, stop := decodeEchoEscapes(next())
		return fmt.Sprintf(spec+"s", dec), j, true, stop
	case 'c':
		first := ""
		if rs := []rune(next()); len(rs) > 0 {
			first = string(rs[0])
		}
		return fmt.Sprintf(spec+"s", first), j, true, false
	case 'd', 'i', 'u', 'o', 'x', 'X':
		n, ok := printfInt(next())
		verb := map[rune]string{'d': "d", 'i': "d", 'u': "d", 'o': "o", 'x': "x", 'X': "X"}[conv]
		return fmt.Sprintf(spec+verb, n), j, ok, false
	case 'e', 'E', 'f', 'F', 'g', 'G':
		a := next()
		f, err := strconv.ParseFloat(a, 64)
		if a == "" {
			f, err = 0, nil
		}
		return fmt.Sprintf(spec+string(conv), f), j, err == nil, false
	}
	return "", j, false, false
}

// printfInt lee un argumento numérico de printf: decimal, `0x…`, octal con
// `0` inicial, vacío (0) o `'c`/`"c`, que vale el código del carácter.
func printfInt(a string) (int64, bool) {
	if a == "" {
		return 0, true
	}
	if a[0] == '\'' || a[0] == '"' {
		if rs := []rune(a[1:]); len(rs) > 0 {
			return int64(rs[0]), true
		}
		return 0, true
	}
	digits := strings.TrimLeft(a, "+-")
	base := 10
	switch {
	case strings.HasPrefix(digits, "0x") || strings.HasPrefix(digits, "0X"):
		base, digits = 16, digits[2:]
	case len(digits) > 1 && digits[0] == '0':
		base = 8
	}
	n, err := strconv.ParseInt(digits, base, 64)
	if strings.HasPrefix(a, "-") {
		n = -n
	}
	return n, err == nil
}
