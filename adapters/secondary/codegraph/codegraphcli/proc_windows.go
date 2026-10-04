//go:build windows

package codegraphcli

import "os/exec"

func detach(cmd *exec.Cmd) {}
