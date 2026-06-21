package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
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

	project := filepath.Base(root)

	active, _ := deps.SessionRepo.Active(project)
	if active != nil {
		fmt.Printf("⚠  Ya hay una sesión activa desde %s\n", active.CreatedAt)
		fmt.Printf("   Ciérrala con: mem session end\n")
		return
	}

	sess, err := deps.SessionRepo.Start(project)
	if err != nil {
		fail("iniciar sesión: %v", err)
	}

	fmt.Printf("✓ Sesión iniciada: %s\n", sess.ID[:8])
	fmt.Println("  Usa 'mem save' durante la sesión para asociar aprendizajes")
}

func cmdSessionEnd(deps *Deps, args []string) {
	fs := flag.NewFlagSet("session end", flag.ContinueOnError)
	summary := fs.String("s", "", "Resumen de la sesión")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)
	sess, err := deps.SessionRepo.Active(project)
	if err != nil {
		fail("%v", err)
	}
	if sess == nil {
		fail("no hay sesión activa para cerrar")
	}

	finalSummary := *summary
	if finalSummary == "" {
		fmt.Print("Resumen de la sesión (o Enter para omitir): ")
		var input string
		fmt.Scanln(&input)
		finalSummary = strings.TrimSpace(input)
	}

	if err := deps.SessionRepo.End(sess.ID, finalSummary); err != nil {
		fail("%v", err)
	}

	fmt.Printf("✓ Sesión %s finalizada\n", sess.ID[:8])
	if finalSummary != "" {
		fmt.Printf("  Resumen: %s\n", finalSummary)
	}
}

func cmdSessionList(deps *Deps, args []string) {
	fs := flag.NewFlagSet("session list", flag.ContinueOnError)
	limit := fs.Int("n", 10, "Número de sesiones")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)
	sessions, err := deps.SessionRepo.Recent(project, *limit)
	if err != nil {
		fail("%v", err)
	}

	if len(sessions) == 0 {
		fmt.Println("Sin sesiones registradas")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tInicio\tFin\tResumen\n")
	fmt.Fprintf(w, "--\t------\t---\t-------\n")
	for _, s := range sessions {
		endStr := "activa"
		if s.EndedAt != nil {
			endStr = *s.EndedAt
		}
		summary := s.Summary
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID[:8], s.CreatedAt, endStr, summary)
	}
	w.Flush()
}
