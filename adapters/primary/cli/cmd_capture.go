package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/domain"
)

func CmdCapture(deps *Deps, args []string) {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	what := fs.String("w", "", "¿Qué se hizo o aprendió?")
	why := fs.String("y", "", "¿Por qué se hizo así?")
	where := fs.String("f", "", "Archivos afectados (separados por coma)")
	learned := fs.String("l", "", "¿Qué se aprendió? (gotchas, edge cases)")
	mtype := fs.String("t", "learning", "Tipo: learning|decision|architecture|bugfix|pattern|discovery")
	interactive := fs.Bool("i", false, "Modo interactivo")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)

	if *interactive || (*what == "" && *why == "" && *learned == "") {
		reader := bufio.NewReader(os.Stdin)
		if *what == "" {
			fmt.Fprint(os.Stderr, "What (¿qué se hizo?): ")
			*what, _ = reader.ReadString('\n')
			*what = strings.TrimSpace(*what)
		}
		if *why == "" {
			fmt.Fprint(os.Stderr, "Why (¿por qué?): ")
			*why, _ = reader.ReadString('\n')
			*why = strings.TrimSpace(*why)
		}
		if *where == "" {
			fmt.Fprint(os.Stderr, "Where (archivos, opcional): ")
			*where, _ = reader.ReadString('\n')
			*where = strings.TrimSpace(*where)
		}
		if *learned == "" {
			fmt.Fprint(os.Stderr, "Learned (¿qué aprendiste?): ")
			*learned, _ = reader.ReadString('\n')
			*learned = strings.TrimSpace(*learned)
		}
	}

	if *what == "" && *learned == "" {
		fail("debes proporcionar al menos --what (-w) o --learned (-l)")
	}

	var contentParts []string
	if *what != "" {
		contentParts = append(contentParts, "**What**: "+*what)
	}
	if *why != "" {
		contentParts = append(contentParts, "**Why**: "+*why)
	}
	if *where != "" {
		contentParts = append(contentParts, "**Where**: "+*where)
	}
	if *learned != "" {
		contentParts = append(contentParts, "**Learned**: "+*learned)
	}
	content := strings.Join(contentParts, "\n")

	memType := domain.ValidMemoryType(*mtype)
	var sessionID string
	sess, _ := deps.SessionRepo.Active(project)
	if sess != nil {
		sessionID = sess.ID
	}

	mem := domain.Memory{
		Project:   project,
		SessionID: sessionID,
		Type:      memType,
		Title:     *what,
		Content:   content,
		Filepath:  *where,
	}

	id, err := deps.MemoryRepo.Insert(&mem)
	if err != nil {
		fail("guardar capture: %v", err)
	}

	fmt.Printf("✓ Capture guardado (id=%d)\n", id)
	if sessionID != "" {
		fmt.Printf("  Sesión activa: %s\n", sessionID[:8])
	}
}
