//go:build windows

package codebasememory

import "os/exec"

// detach es no-op en Windows: exec.Command sin ligar handles ya arranca un
// proceso independiente; el refresco es best-effort de todos modos.
func detach(cmd *exec.Cmd) {}
