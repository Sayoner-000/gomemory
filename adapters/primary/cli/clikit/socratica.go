package clikit

import (
	"io"

	"mem/adapters/primary/console"
	"mem/application/ports"
)

// SelectSocraticHarness presents an available catalog entry while keeping the
// CLI toolkit independent from the Socratica slice that supplies the catalog.
func SelectSocraticHarness(catalog []ports.SocraticHarness, ui console.UI, in io.Reader, out io.Writer, unavailable func(label, reason string), exclude ...string) (string, error) {
	var opts []console.Option
	for _, h := range catalog {
		skipped := false
		for _, id := range exclude {
			if h.ID == id {
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		if !h.Available {
			if unavailable != nil {
				unavailable(h.Label, h.Reason)
			}
			continue
		}
		opts = append(opts, console.Option{Value: h.ID, Label: h.Label, Hint: "preguntas, reescritura y ejecución"})
	}
	if len(opts) == 0 {
		return "", ports.ErrNoGenerator
	}
	opts = append(opts, console.Option{Value: "cancel", Label: "Cancelar", Hint: "conservar el original"})
	if ui == nil {
		ui = console.NewPlain(in, out)
	}
	choice, err := ui.Select("Socratica · Elige el harness para este flujo", opts)
	if choice == "cancel" {
		return "", console.ErrCanceled
	}
	return choice, err
}
