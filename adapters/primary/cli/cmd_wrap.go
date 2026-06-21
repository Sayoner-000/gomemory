package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mem/domain"
)

func CmdWrap(deps *Deps, args []string) {
	fs := flag.NewFlagSet("wrap", flag.ContinueOnError)
	autoSession := fs.Bool("s", true, "Auto-iniciar sesión si no hay una activa")
	fs.Parse(args)

	command := fs.Args()
	if len(command) == 0 {
		fail("uso: mem wrap [-s] <comando> [args...]\n  Ejemplo: mem wrap opencode")
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)

	if *autoSession {
		active, _ := deps.SessionRepo.Active(project)
		if active == nil {
			sess, err := deps.SessionRepo.Start(project)
			if err == nil {
				fmt.Fprintf(os.Stderr, "✓ Sesión auto-iniciada: %s\n", sess.ID[:8])
			}
		}
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	fmt.Fprint(os.Stderr, "\n━━━ ¿Guardar algo en memoria? ━━━\n")
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprint(os.Stderr, "¿Guardar? (s/N): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer == "s" || answer == "S" || answer == "y" || answer == "Y" {
		saveInteractive(reader, deps, project)
	}

	os.Exit(exitCode)
}

func saveInteractive(reader *bufio.Reader, deps *Deps, project string) {
	fmt.Fprint(os.Stderr, "Título: ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimSpace(title)

	fmt.Fprint(os.Stderr, "Tipo (learning/decision/bugfix/pattern/architecture) [learning]: ")
	typeStr, _ := reader.ReadString('\n')
	typeStr = strings.TrimSpace(typeStr)
	if typeStr == "" {
		typeStr = "learning"
	}
	memType := domain.ValidMemoryType(typeStr)

	fmt.Fprint(os.Stderr, "Contenido: ")
	content, _ := reader.ReadString('\n')
	content = strings.TrimSpace(content)

	if content == "" {
		fmt.Fprintln(os.Stderr, "⚠  Contenido vacío, no se guardó nada")
		return
	}

	var sessionID string
	sess, _ := deps.SessionRepo.Active(project)
	if sess != nil {
		sessionID = sess.ID
	}

	mem := domain.Memory{
		Project:   project,
		SessionID: sessionID,
		Type:      memType,
		Title:     title,
		Content:   content,
	}

	id, err := deps.MemoryRepo.Insert(&mem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al guardar: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✓ Memoria guardada (id=%d)\n", id)
}
