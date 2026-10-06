// Package assets contiene los recursos visuales que viajan con el binario.
package assets

import _ "embed"

// TerminalLogo adapta la gota y el aro de gomemory_dark.png a caracteres.
//
//go:embed gomemory-terminal.txt
var TerminalLogo string
