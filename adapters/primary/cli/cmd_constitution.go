package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"mem/application/usecases"
	"mem/domain"
)

// CmdConstitution y CmdRules son atajos ergonómicos sobre `mem docs show`.
// La constitución y las reglas se consultan a menudo; tener que recordar el
// subcomando para lo frecuente es fricción sin ganancia.
//
// Comparten toda la maquinaria con `mem docs`: no hay una segunda resolución
// que pueda divergir.
func CmdConstitution(deps *Deps, args []string) {
	if err := runPinnedShortcut(deps, "constitution", args, os.Stdout, os.Stderr); err != nil {
		fail("%v", err)
	}
}

func CmdRules(deps *Deps, args []string) {
	if err := runPinnedShortcut(deps, "rules", args, os.Stdout, os.Stderr); err != nil {
		fail("%v", err)
	}
}

// runPinnedShortcut resuelve un documento fijado por su alias y lo escribe en
// stdout. Con --sync, además lo refleja en el archivo que espera spec-kit.
func runPinnedShortcut(deps *Deps, alias string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(alias, flag.ContinueOnError)
	fs.SetOutput(stderr)
	sync := fs.Bool("sync", false, "escribir también .specify/memory/constitution.md si el proyecto usa spec-kit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	doc, ok := domain.PinnedDocByAlias(alias)
	if !ok {
		return fmt.Errorf("documento %q no está en el catálogo", alias)
	}

	_, topics, ok := seedDeps(deps)
	if !ok {
		return fmt.Errorf("no hay acceso a la memoria del proyecto")
	}

	res, err := usecases.ResolvePinnedDoc(topics, deps.Project, doc.TopicKey, embeddedTemplate(doc.Template))
	if err != nil {
		return err
	}
	if !res.FromMemory {
		fmt.Fprintf(stderr, "⚠️  No hay %s en la memoria de este proyecto; se muestra el contenido por defecto.\n", doc.Label)
	}

	if _, err := io.WriteString(stdout, res.Content); err != nil {
		return err
	}

	if *sync {
		return syncSpeckitConstitution(deps.Root, res.Content, stderr)
	}
	return nil
}

// syncSpeckitConstitution refleja el contenido en .specify/memory/constitution.md.
//
// NUNCA crea .specify/: misma comprobación que InstallSpeckitExtension. Un
// proyecto que no usa spec-kit no debe encontrarse con una estructura nueva por
// haber pedido sincronizar.
func syncSpeckitConstitution(root, contenido string, stderr io.Writer) error {
	info, err := os.Stat(filepath.Join(root, ".specify"))
	if err != nil || !info.IsDir() {
		fmt.Fprintln(stderr, "ℹ️  Este proyecto no usa spec-kit; no se sincronizó ningún archivo.")
		return nil
	}

	dir := filepath.Join(root, ".specify", "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("preparar %s: %w", dir, err)
	}
	destino := filepath.Join(dir, "constitution.md")
	if err := os.WriteFile(destino, []byte(contenido), 0o644); err != nil {
		return fmt.Errorf("escribir %s: %w", destino, err)
	}
	fmt.Fprintf(stderr, "✅ %s actualizado\n", destino)
	return nil
}
