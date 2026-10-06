package console

import (
	"os"
	"strings"
	"testing"

	"mem/assets"
)

func TestPortableInstallersUseCurrentTerminalMark(t *testing.T) {
	for _, file := range []string{"../../../scripts/install.sh", "../../../scripts/install.ps1"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), strings.TrimSuffix(assets.TerminalLogo, "\n")) {
			t.Errorf("%s conserva una marca diferente", file)
		}
	}
}
