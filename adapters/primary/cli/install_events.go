package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"mem/adapters/primary/console"
)

// El puente separa la salida histórica del instalador de su contrato NDJSON.
// El subproceso conserva la misma lógica de instalación y sus códigos de salida.
const installEventPrefix = "\x1egomemory-event:"

type installEvent struct {
	Version  int    `json:"contract_version"`
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Manual   string `json:"manual,omitempty"`
	ExitCode int    `json:"exit_code"`
}

func writeInstallEvent(out io.Writer, event installEvent) { _ = json.NewEncoder(out).Encode(event) }

func eventFromStep(step console.StepResult) installEvent {
	status := "ok"
	if step.Status == console.StepWarn {
		status = "warn"
	}
	if step.Status == console.StepFail {
		status = "fail"
	}
	return installEvent{Version: 1, Type: "step", Name: step.Name, Status: status, Detail: step.Detail, Manual: step.Manual}
}

func bridgeInstallEvents(input io.Reader, events, logs io.Writer) (int, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	warnings := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, installEventPrefix) {
			fmt.Fprintln(logs, line)
			continue
		}
		var event installEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, installEventPrefix)), &event); err != nil {
			return warnings, err
		}
		if event.Version != 1 || event.Type != "step" || event.Name == "" || (event.Status != "ok" && event.Status != "warn" && event.Status != "fail") {
			return warnings, fmt.Errorf("evento de instalación inválido")
		}
		if event.Status != "ok" {
			warnings++
		}
		writeInstallEvent(events, event)
	}
	return warnings, scanner.Err()
}

func runInstallEvents(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	self, err := os.Executable()
	if err != nil {
		writeInstallEvent(os.Stdout, installEvent{Version: 1, Type: "complete", Status: "fail", Detail: err.Error(), ExitCode: 1})
		return 1
	}
	childArgs := []string{"install", "--yes"}
	for _, arg := range args {
		if arg != "--events" {
			childArgs = append(childArgs, arg)
		}
	}
	cmd := exec.CommandContext(ctx, self, childArgs...)
	configureInstallProcessTree(cmd)
	cmd.Env = append(os.Environ(), "GOMEMORY_INSTALL_EVENT_CHILD=1", "CI=1", "GOMEMORY_NO_MOTION=1")
	cmd.Stdout = os.Stderr
	pipe, err := cmd.StderrPipe()
	if err != nil {
		writeInstallEvent(os.Stdout, installEvent{Version: 1, Type: "complete", Status: "fail", Detail: err.Error(), ExitCode: 1})
		return 1
	}
	writeInstallEvent(os.Stdout, installEvent{Version: 1, Type: "start", Name: "Instalación"})
	if err := cmd.Start(); err != nil {
		writeInstallEvent(os.Stdout, installEvent{Version: 1, Type: "complete", Status: "fail", Detail: err.Error(), ExitCode: 1})
		return 1
	}
	warnings, streamErr := bridgeInstallEvents(pipe, os.Stdout, os.Stderr)
	if streamErr != nil {
		if stopErr := stopInstallProcessTree(cmd); stopErr != nil && stopErr != os.ErrProcessDone {
			streamErr = fmt.Errorf("%w; detener instalación: %v", streamErr, stopErr)
		}
	}
	err = cmd.Wait()
	status, code, detail := "ok", 0, ""
	if warnings > 0 {
		status = "warn"
	}
	if err != nil {
		status, code, detail = "fail", 1, err.Error()
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
	}
	if streamErr != nil {
		status, code, detail = "fail", 1, streamErr.Error()
	}
	if ctx.Err() != nil {
		status, code, detail = "fail", 130, "Instalación cancelada"
	}
	writeInstallEvent(os.Stdout, installEvent{Version: 1, Type: "complete", Status: status, Detail: detail, ExitCode: code})
	return code
}
