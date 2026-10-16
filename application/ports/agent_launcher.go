package ports

import (
	"context"
	"errors"
)

var (
	ErrLaunchFailed  = errors.New("agent launch failed")
	ErrConfirmFailed = errors.New("dispatch confirmation failed")
)

type LaunchRequest struct {
	Client, Prompt string
	Env            map[string]string
}
type AgentLauncher interface {
	Launch(context.Context, LaunchRequest, func() error) error
}
