package cli

import (
	"io"

	"mem/domain"
)

// terminalSafe writes through domain.StripTerminalControls, for plain-text
// reports whose fields come from agents or storage and carry no styling of
// their own. Stripping is per character, so how fmt splits writes is moot.
type terminalSafe struct{ w io.Writer }

func (t terminalSafe) Write(p []byte) (int, error) {
	if _, err := io.WriteString(t.w, domain.StripTerminalControls(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}
