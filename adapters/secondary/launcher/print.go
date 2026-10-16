package launcher

import (
	"context"
	"fmt"
	"io"

	"mem/application/ports"
)

// Print hands the prompt over by writing it once, then confirms. Without raw,
// the line starts with ">" so that pasting it into an agent with interception
// on is forwarded instead of captured again (G5).
type Print struct {
	w   io.Writer
	raw bool
}

func NewPrint(w io.Writer, raw bool) *Print { return &Print{w: w, raw: raw} }

func (l *Print) Launch(_ context.Context, req ports.LaunchRequest, onStarted func() error) error {
	prefix := ">"
	if l.raw {
		prefix = ""
	}
	if _, err := fmt.Fprintf(l.w, "%s%s\n", prefix, req.Prompt); err != nil {
		return fmt.Errorf("%w: %v", ports.ErrLaunchFailed, err)
	}
	if err := onStarted(); err != nil {
		return fmt.Errorf("%w: %v", ports.ErrConfirmFailed, err)
	}
	return nil
}
