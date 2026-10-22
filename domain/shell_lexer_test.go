package domain

import (
	"reflect"
	"testing"
)

func TestSplitShellCommands(t *testing.T) {
	cases := []struct {
		in   string
		want [][]string
	}{
		{"", nil},
		{"   ", nil},
		{"ls", [][]string{{"ls"}}},
		{"ls -la /tmp", [][]string{{"ls", "-la", "/tmp"}}},
		{"ls && rm -rf build", [][]string{{"ls"}, {"rm", "-rf", "build"}}},
		{"a; b || c | d", [][]string{{"a"}, {"b"}, {"c"}, {"d"}}},
		{"a\nb", [][]string{{"a"}, {"b"}}},
		{`echo "rm -rf /"`, [][]string{{"echo", "rm -rf /"}}},
		{`echo 'a && b'`, [][]string{{"echo", "a && b"}}},
		{`echo "it's ok"`, [][]string{{"echo", "it's ok"}}},
		{`echo 'say "hi"'`, [][]string{{"echo", `say "hi"`}}},
		{`echo a\ b`, [][]string{{"echo", "a b"}}},
		{`git commit -m "x; y"`, [][]string{{"git", "commit", "-m", "x; y"}}},
		{"a&&b", [][]string{{"a"}, {"b"}}},
		{";;", nil},
		{`echo "unterminated`, [][]string{{"echo", "unterminated"}}},
		// B-002: & simple separa comandos; en una redirección, no.
		{"true & rm -rf build", [][]string{{"true"}, {"rm", "-rf", "build"}}},
		{"sleep 1&rm x", [][]string{{"sleep", "1"}, {"rm", "x"}}},
		{"go test 2>&1", [][]string{{"go", "test"}}},
		{"cmd >&2", [][]string{{"cmd"}}},
		{"cmd &>log", [][]string{{"cmd"}}},
		{`echo "a & b"`, [][]string{{"echo", "a & b"}}},
		{"server &", [][]string{{"server"}}},
		// B-001: barra invertida + salto de línea es continuación.
		{"rm -rf \\\nbuild", [][]string{{"rm", "-rf", "build"}}},
		{"r\\\nm -r\\\nf build", [][]string{{"rm", "-rf", "build"}}},
		{"echo \"a\\\nb\"", [][]string{{"echo", "ab"}}},
		{"echo 'a\\\nb'", [][]string{{"echo", "a\\\nb"}}},
		// A-002: el cuerpo de un heredoc es dato; lo posterior al delimitador, no.
		{"cat <<EOF > doc.md\nrm -rf /\nEOF", [][]string{{"cat"}}},
		{"cat <<'EOF'\nrm -rf /\nEOF\nrm -rf build", [][]string{{"cat"}, {"rm", "-rf", "build"}}},
		{"cat <<-\"END\"\n\trm -rf /\n\tEND\nls", [][]string{{"cat"}, {"ls"}}},
		{"cat <<A <<B\nx\nA\ny\nB\nls", [][]string{{"cat"}, {"ls"}}},
		{"cat <<EOF\nrm -rf /", [][]string{{"cat"}}},
		{"cat <<< 'rm -rf /'\nls", [][]string{{"cat"}, {"ls"}}},
		// H-01: las redirecciones salen del argv estén donde estén; lo
		// citado o escapado sigue siendo argumento.
		{">/dev/null rm -rf x", [][]string{{"rm", "-rf", "x"}}},
		{"> /dev/null rm -rf x", [][]string{{"rm", "-rf", "x"}}},
		{"2>/dev/null 1>&2 rm -rf x", [][]string{{"rm", "-rf", "x"}}},
		{"2> err <in >>log rm -rf x", [][]string{{"rm", "-rf", "x"}}},
		{"&>/dev/null rm x", [][]string{{"rm", "x"}}},
		{">&- rm x", [][]string{{"rm", "x"}}},
		{">| out rm x", [][]string{{"rm", "x"}}},
		{"{fd}>log rm x", [][]string{{"rm", "x"}}},
		{"<< EOF rm x\nbody\nEOF", [][]string{{"rm", "x"}}},
		{"<<- EOF rm x\n\tEOF", [][]string{{"rm", "x"}}},
		{"rm -rf x>/dev/null", [][]string{{"rm", "-rf", "x"}}},
		{`echo ">" "2>x" \> a`, [][]string{{"echo", ">", "2>x", ">", "a"}}},
		{`echo "2">x`, [][]string{{"echo", "2"}}},
		{"ls >$(rm -rf x)", [][]string{{"ls"}, {"rm", "-rf", "x"}}},
		// H-02: un heredoc sin delimitador citado ejecuta sus sustituciones;
		// citado o escapado, es texto.
		{"cat <<EOF\n$(rm -rf x)\nEOF", [][]string{{"cat"}, {"rm", "-rf", "x"}}},
		{"cat <<EOF\na `rm -rf x` b\nEOF", [][]string{{"cat"}, {"rm", "-rf", "x"}}},
		{"cat <<EOF\n${V:-$(rm -rf x)}\nEOF", [][]string{{"cat"}, {"rm", "-rf", "x"}}},
		{"cat <<EOF\n\\\\$(rm -rf x)\nEOF", [][]string{{"cat"}, {"rm", "-rf", "x"}}},
		{"cat <<EOF\n\\$(rm -rf x) \\`rm -rf y\\`\nEOF", [][]string{{"cat"}}},
		{"cat <<'EOF'\n$(rm -rf x)\nEOF", [][]string{{"cat"}}},
		{"cat <<\\EOF\n$(rm -rf x)\nEOF", [][]string{{"cat"}}},
		{"cat <<\"EOF\"\n`rm -rf x`\nEOF", [][]string{{"cat"}}},
		// R3-01: comillas ANSI-C y localizadas.
		{`bash -c $'rm -rf /tmp/x'`, [][]string{{"bash", "-c", "rm -rf /tmp/x"}}},
		{`bash -c $"rm -rf /tmp/x"`, [][]string{{"bash", "-c", "rm -rf /tmp/x"}}},
		{`$'\x72\x6d' -rf /`, [][]string{{"rm", "-rf", "/"}}},
		{`$'\162\155' -rf /`, [][]string{{"rm", "-rf", "/"}}},
		{`echo $'a\'b\tc\q'`, [][]string{{"echo", "a'b\tc\\q"}}},
		{`echo $HOME`, [][]string{{"echo", "$HOME"}}},
		{`echo $`, [][]string{{"echo", "$"}}},
	}
	for _, c := range cases {
		if got := SplitShellCommands(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitShellCommands(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

// El texto de un herestring llega como entrada estándar del comando, no
// como argumento (H-01).
func TestSplitShellHerestringIsStdin(t *testing.T) {
	cases := map[string][]string{
		"bash <<< 'rm -rf x'":   {"rm -rf x"},
		"bash <<<'rm -rf x'":    {"rm -rf x"},
		"bash 0<<< \"ls\" <<<y": {"ls", "y"},
	}
	for in, want := range cases {
		cmds := splitShell(in, 0)
		if len(cmds) != 1 || !reflect.DeepEqual(cmds[0].Tokens, []string{"bash"}) || !reflect.DeepEqual(cmds[0].Stdin, want) {
			t.Errorf("splitShell(%q) = %#v, want bash con Stdin %q", in, cmds, want)
		}
	}
}
