package cli

import (
	"bufio"
	"bytes"
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

// consumeNativeInstall usa el contrato ya emitido por este mismo binario.
// No infiere éxito ni progreso a partir de frases de la salida histórica.
func consumeNativeInstall(input io.Reader, flow *console.Flow) (installEvent, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	scanner.Split(scanNDJSONLines)
	var complete installEvent
	warnings, closed := false, false
	for scanner.Scan() {
		// Mismo contrato y orden de comprobaciones que EventStream en
		// installer/src/core.ts (S-003).
		if strings.TrimSpace(scanner.Text()) == "" {
			return complete, fmt.Errorf("línea en blanco fuera del contrato NDJSON")
		}
		if closed {
			return complete, fmt.Errorf("evento después del cierre")
		}
		var event installEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return complete, err
		}
		if event.Version != 1 {
			return complete, fmt.Errorf("contrato de instalación incompatible")
		}
		switch event.Type {
		case "start":
			flow.Progress("Configurando el proyecto")
		case "step":
			step := console.StepResult{Name: event.Name, Detail: event.Detail, Manual: event.Manual}
			if step.Name == "" {
				return complete, fmt.Errorf("paso sin nombre")
			}
			switch event.Status {
			case "ok":
				step.Status = console.StepOK
			case "warn":
				step.Status = console.StepWarn
				warnings = true
			case "fail":
				step.Status = console.StepFail
				warnings = true
			default:
				return complete, fmt.Errorf("estado de paso inválido")
			}
			flow.Done(step)
			flow.Progress("Configurando el proyecto")
		case "complete":
			if event.Status != "ok" && event.Status != "warn" && event.Status != "fail" {
				return event, fmt.Errorf("estado de cierre inválido")
			}
			if event.ExitCode < 0 || (event.Status == "fail" && event.ExitCode == 0) || (warnings && event.Status == "ok") {
				return event, fmt.Errorf("cierre inconsistente con los resultados")
			}
			complete, closed = event, true
		default:
			return complete, fmt.Errorf("tipo de evento desconocido")
		}
	}
	if err := scanner.Err(); err != nil {
		return complete, err
	}
	if !closed {
		return complete, fmt.Errorf("instalación sin evento de cierre")
	}
	return complete, nil
}

// scanNDJSONLines es bufio.ScanLines salvo que un evento final sin salto de
// línea se rechaza como truncado, igual que el instalador TypeScript.
func scanNDJSONLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) > 0 && bytes.IndexByte(data, '\n') < 0 {
		return 0, nil, fmt.Errorf("evento truncado: falta salto de línea de cierre")
	}
	return bufio.ScanLines(data, atEOF)
}

// nativeLogs mantiene un diagnóstico acotado sin mezclarlo con la presentación.
type nativeLogs struct{ text []byte }

func (b *nativeLogs) Write(p []byte) (int, error) {
	b.text = append(b.text, p...)
	if len(b.text) > 64*1024 {
		b.text = append([]byte(nil), b.text[len(b.text)-64*1024:]...)
	}
	return len(p), nil
}

func runNativeInstall(target string, selection installSelection, hasGlobal bool) int {
	flow := console.NewFlow(os.Stdout, console.DetectEnv(), "Configurar proyecto")
	defer flow.Stop()
	flow.Next("mem tui")
	self, err := os.Executable()
	if err != nil {
		flow.Done(console.StepResult{Name: "Binario", Status: console.StepFail, Detail: err.Error()})
		flow.End("fail")
		return 1
	}
	if !hasGlobal && selection.binary == "global" {
		err := installGlobalBinary(self)
		step := console.StepResult{Name: "Binario global", Status: console.StepOK}
		if err != nil {
			step.Status, step.Detail, step.Manual = console.StepWarn, err.Error(), "scripts/install.sh"
		}
		flow.Done(step)
	}
	agents := strings.Join(selection.agents, ",")
	if agents == "" {
		agents = "none"
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "install", target, "--events", "--yes", "--agents", agents, "--scope", selection.scope)
	cmd.Cancel = func() error { return requestInstallStop(cmd) }
	var logs nativeLogs
	cmd.Stderr = &logs
	pipe, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		flow.Done(console.StepResult{Name: "Instalación", Status: console.StepFail, Detail: err.Error()})
		flow.End("fail")
		return 1
	}
	complete, protocolErr := consumeNativeInstall(pipe, flow)
	if protocolErr != nil {
		if stopErr := requestInstallStop(cmd); stopErr != nil && stopErr != os.ErrProcessDone {
			protocolErr = fmt.Errorf("%w; cancelar puente: %v", protocolErr, stopErr)
		}
	}
	waitErr := cmd.Wait()
	if protocolErr != nil || waitErr != nil || complete.ExitCode != 0 {
		detail := complete.Detail
		if protocolErr != nil {
			detail = protocolErr.Error()
		}
		if ctx.Err() != nil {
			detail = "Cancelado por la persona"
		}
		if len(logs.text) > 0 {
			detail += "\n" + strings.TrimSpace(string(logs.text))
		}
		flow.Done(console.StepResult{Name: "Instalación", Status: console.StepFail, Detail: detail})
		flow.End("fail")
		if ctx.Err() != nil {
			return 130
		}
		if complete.ExitCode > 0 {
			return complete.ExitCode
		}
		return 1
	}
	flow.End(complete.Status)
	return 0
}
