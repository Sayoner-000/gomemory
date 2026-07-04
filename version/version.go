// Package version es la única fuente de verdad para la versión de gomemory.
package version

// Version se usa en el server MCP, en "mem version" y en el instalador.
// Es var (no const) para permitir -ldflags "-X mem/version.Version=vX.Y.Z" en releases.
var Version = "1.10.0"
