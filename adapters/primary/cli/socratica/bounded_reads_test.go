package socratica

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"mem/adapters/primary/cli/clikit"
)

func TestSocraticaRefine_StdinAcotado(t *testing.T) {
	e := newCLIEnv(t)
	big := strings.Repeat("a", socraticaPromptMax+1)
	var errOut bytes.Buffer
	code := runSocratica(e.deps, []string{"refine", "-"}, socraticaIO{in: bufio.NewReader(strings.NewReader(big)), out: io.Discard, err: &errOut})
	if code != socraticaExitUsage || !strings.Contains(errOut.String(), clikit.ErrPayloadTooLarge.Error()) {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
	if e.gen.seen != nil {
		t.Fatal("llamó al generador con una entrada fuera del tope")
	}
}
