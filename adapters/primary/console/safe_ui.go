package console

import "mem/domain"

// safeUI paints titles, labels and hints without control sequences, whichever
// mode draws them: options can carry generated text (Socratica questions) and
// a selection returns the option's Value, so the caller's data stays exact.
type safeUI struct{ inner UI }

func (s safeUI) MultiSelect(title string, opts []Option) ([]string, error) {
	return s.inner.MultiSelect(domain.StripTerminalControls(title), safeOptions(opts))
}
func (s safeUI) Select(title string, opts []Option) (string, error) {
	return s.inner.Select(domain.StripTerminalControls(title), safeOptions(opts))
}
func (s safeUI) Confirm(title string, def bool) (bool, error) {
	return s.inner.Confirm(domain.StripTerminalControls(title), def)
}
func (s safeUI) TypedConfirm(title, word string) (bool, error) {
	return s.inner.TypedConfirm(domain.StripTerminalControls(title), word)
}

func safeOptions(opts []Option) []Option {
	out := make([]Option, len(opts))
	for i, o := range opts {
		o.Label, o.Hint = domain.StripTerminalControls(o.Label), domain.StripTerminalControls(o.Hint)
		out[i] = o
	}
	return out
}
