package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"mem/adapters/primary/console"
)

func CmdSession(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("subcomando requerido: start, end, list\nEjemplo: mem session start")
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "start":
		cmdSessionStart(deps, subArgs)
	case "end":
		cmdSessionEnd(deps, subArgs)
	case "list":
		cmdSessionList(deps, subArgs)
	default:
		fail("subcomando desconocido: %s (opciones: start, end, list)", sub)
	}
}

func cmdSessionStart(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := deps.ProjectRepo.Key(root)

	// OpenCode llama a `mem session start --conversation=<id>` en
	// session.created: es su inicio de conversación (feature 035, FR-017), haya
	// o no una sesión de memoria activa.
	beginConversation(deps, root, conversationFlag(args))

	active, _ := deps.SessionRepo.Active(project)
	if active != nil {
		humanf("⚠  Ya hay una sesión activa desde %s\n", active.CreatedAt)
		humanf("   Ciérrala con: mem session end\n")
		return
	}

	sess, err := deps.SessionRepo.Start(project)
	if err != nil {
		fail("iniciar sesión: %v", err)
	}

	humanf("✓ Sesión iniciada: %s\n", sess.ID[:8])
	humanln("  Usa 'mem save' durante la sesión para asociar aprendizajes")
}

func cmdSessionEnd(deps *Deps, args []string) {
	fs := newFlagSet("session end", flag.ContinueOnError)
	summary := fs.String("s", "", "Resumen de la sesión")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := deps.ProjectRepo.Key(root)
	sess, err := deps.SessionRepo.Active(project)
	if err != nil {
		fail("%v", err)
	}
	if sess == nil {
		endConversation(root) // cierre explícito también limpia un registro huérfano
		fail("no hay sesión activa para cerrar")
	}

	finalSummary := *summary
	if finalSummary == "" {
		fmt.Print("Resumen de la sesión (o Enter para omitir): ")
		var input string
		_, _ = fmt.Scanln(&input)
		finalSummary = strings.TrimSpace(input)
	}

	if err := deps.SessionRepo.End(sess.ID, finalSummary); err != nil {
		fail("%v", err)
	}
	endConversation(root)

	humanf("✓ Sesión %s finalizada\n", sess.ID[:8])
	if finalSummary != "" {
		humanf("  Resumen: %s\n", finalSummary)
	}
}

func cmdSessionList(deps *Deps, args []string) {
	fs := newFlagSet("session list", flag.ContinueOnError)
	limit := fs.Int("n", 10, "Número de sesiones")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := deps.ProjectRepo.Key(root)
	sessions, err := deps.SessionRepo.Recent(project, *limit)
	if err != nil {
		fail("%v", err)
	}

	if len(sessions) == 0 {
		humanln("Sin sesiones registradas. Inicia una con: mem session start")
		return
	}

	rows := make([][]string, 0, len(sessions))
	for _, s := range sessions {
		endStr := "activa"
		if s.EndedAt != nil {
			endStr = *s.EndedAt
		}
		summary := s.Summary
		summary = ansi.Truncate(summary, 50, "...")
		rows = append(rows, []string{s.ID[:8], s.CreatedAt, endStr, summary})
	}
	fmt.Fprint(os.Stdout, formatMemoryRows(console.DetectEnv(), []string{"ID", "Inicio", "Fin", "Resumen"}, rows))
}

// conversationFlag extrae --conversation=<id> de args ("" si no está).
func conversationFlag(args []string) string {
	const prefix = "--conversation="
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
	}
	return ""
}
