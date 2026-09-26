//go:build windows

package cli

import "os/exec"

// detach es no-op en Windows: un proceso lanzado sin ligar handles ya es
// independiente del padre.
func detach(cmd *exec.Cmd) {}
