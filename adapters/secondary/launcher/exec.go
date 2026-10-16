// Package launcher implements ports.AgentLauncher: it hands an approved prompt
// to an agent exactly once (research R4).
package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"mem/application/ports"
)

// agentArgs is how each agent takes an initial message (research R4).
func agentArgs(client, prompt string) ([]string, bool) {
	switch client {
	case "claude", "codex":
		return []string{prompt}, true
	case "opencode":
		return []string{"--prompt", prompt}, true
	}
	return nil, false
}

// Exec starts the agent attached to the terminal, calls onStarted right after
// Start (where the use case writes the ack) and blocks until the agent exits,
// so mem never competes with it for the terminal.
type Exec struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

// NewExec inherits the process stdio when a stream is nil.
func NewExec(stdin io.Reader, stdout, stderr io.Writer) *Exec {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Exec{stdin: stdin, stdout: stdout, stderr: stderr}
}

func (l *Exec) Launch(ctx context.Context, req ports.LaunchRequest, onStarted func() error) error {
	args, ok := agentArgs(req.Client, req.Prompt)
	if !ok {
		return fmt.Errorf("%w: agente desconocido %q", ports.ErrLaunchFailed, req.Client)
	}
	// Not CommandContext: cancelling mem must never kill an agent the person
	// is already working in.
	cmd := exec.Command(req.Client, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = l.stdin, l.stdout, l.stderr
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", ports.ErrLaunchFailed, err)
	}
	confirmErr := onStarted()
	waitErr := cmd.Wait()
	if confirmErr != nil {
		return errors.Join(ports.ErrConfirmFailed, confirmErr)
	}
	// The agent ran: its own exit status (for example, the person quitting
	// with Ctrl-C) is not a dispatch failure.
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return waitErr
	}
	return nil
}
