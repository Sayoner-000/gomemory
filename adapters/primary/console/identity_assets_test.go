package console

import (
	"bytes"
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

// Windows PowerShell 5.1 lee un .ps1 sin BOM como ANSI: los bytes UTF-8 del
// banner y de las marcas se decodifican como comillas tipográficas y el
// instalador deja de parsear. Todo .ps1 con texto no ASCII lleva BOM.
func TestPowerShellScriptsWithNonASCIIHaveUTF8BOM(t *testing.T) {
	for _, file := range []string{"../../../scripts/install.ps1", "../../../scripts/audit_037_windows_smoke.ps1"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		ascii := true
		for _, b := range content {
			if b >= 0x80 {
				ascii = false
				break
			}
		}
		if !ascii && !bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
			t.Errorf("%s tiene texto no ASCII sin BOM UTF-8", file)
		}
	}
}
