package domain

import "time"

const (
	DefaultHistoryLimit     = 20
	PendingMaxAge           = 7 * 24 * time.Hour
	TerminalRetention       = 24 * time.Hour
	ContinuationWindow      = 30 * time.Minute
	LowInfoMaxWords         = 3
	DefaultGeneratorTimeout = 120 * time.Second
	DefaultQueryTimeout     = 5 * time.Second
)

type SocraticaCapabilities struct{ NamedCommand, Interception, Clipboard, InputRestore, Pane bool }

// No host capability is enabled until acceptance evidence has been collected.
func CapabilitiesForSocratica(client string) SocraticaCapabilities { return SocraticaCapabilities{} }
