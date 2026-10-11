package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"mem/adapters/primary/console"
	"mem/domain"
)

// CmdSanitize implementa `mem sanitize` (spec 037, US2): vista previa por
// defecto y saneamiento explícito con --apply del proyecto actual. No hay
// modo "todos los proyectos" ni ruta arbitraria a una base: el alcance es
// siempre el proyecto resuelto.
//
// Códigos: 0 vista previa o limpieza verificada; 1 fallo, timeout o
// pending_cleanup; 2 argumentos inválidos (no se inicia nada).
func CmdSanitize(deps *Deps, args []string) {
	fs := newFlagSet("sanitize", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "Aplicar el saneamiento (sin esta opción solo se muestra la vista previa)")
	asJSON := fs.Bool("json", false, "Emitir el informe como un objeto JSON")
	timeout := fs.Duration("timeout", 60*time.Second, "Tiempo máximo de la operación (p. ej. 60s, 5m)")
	if err := fs.Parse(args); err != nil {
		return
	}
	if fs.NArg() > 0 || *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "Error: uso: mem sanitize [--apply] [--json] [--timeout <duración positiva>]")
		exitProcess(2)
		return
	}
	if deps.Sanitizer == nil {
		sanitizeDiagnostic(domain.SanitizeCodeFailed, "el almacén del proyecto no está disponible")
		exitProcess(1)
		return
	}
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		sanitizeDiagnostic(domain.SanitizeCodeFailed, "no se encontró el proyecto actual")
		exitProcess(1)
		return
	}
	project := deps.ProjectRepo.Key(root)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	var report domain.SanitizeReport
	if *apply {
		report, err = deps.Sanitizer.Apply(ctx, project)
	} else {
		report, err = deps.Sanitizer.Preview(ctx, project)
	}

	if *asJSON {
		data, merr := json.MarshalIndent(report, "", "  ")
		if merr == nil {
			fmt.Println(string(data))
		}
	} else {
		printSanitizeReport(report)
	}
	if err != nil {
		code := domain.SanitizeCodeFailed
		var se *domain.SanitizeError
		if errors.As(err, &se) {
			code = se.Code
		}
		sanitizeDiagnostic(code, sanitizeHint(report, code))
		exitProcess(1)
		return
	}
}

// sanitizeDiagnostic escribe un diagnóstico estable en stderr. Nunca incluye
// la causa cruda: puede venir del driver o del contenido de un archivo.
func sanitizeDiagnostic(code, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", code, msg)
}

func sanitizeHint(report domain.SanitizeReport, code string) string {
	for _, p := range report.Pending {
		if p.Code == code {
			return p.Action
		}
	}
	switch code {
	case domain.SanitizeCodeTimeout:
		return "se agotó el tiempo; repite con un --timeout mayor"
	case domain.SanitizeCodeBusy:
		return "otra conexión mantiene la base ocupada; detén los clientes de gomemory y repite"
	default:
		return "el saneamiento no terminó; repite mem sanitize --apply"
	}
}

// printSanitizeReport muestra el informe para personas, sin texto de los
// registros: solo categorías, identificadores y estado.
func printSanitizeReport(r domain.SanitizeReport) {
	mode := "vista previa"
	if r.Mode == domain.SanitizeModeApply {
		mode = "aplicación"
	}
	headers := []string{"Categoría", "Registros", "Identificadores"}
	rows := make([][]string, 0, len(r.Categories))
	for _, c := range r.Categories {
		rows = append(rows, []string{string(c.Category), strconv.Itoa(c.AffectedCount), summarizeIDs(c.RecordIDs)})
	}
	title := "Saneamiento de privacidad · " + mode
	if len(rows) == 0 {
		humanf("%s: sin registros que sanear (estado %s)\n", title, r.Status)
	} else if !printListPanel("", title, string(r.Status), headers, rows) {
		humanf("%s (estado %s)\n", title, r.Status)
		fmt.Fprint(os.Stdout, formatMemoryRows(console.DetectEnv(), headers, rows))
	}
	if r.Mode == domain.SanitizeModeApply {
		humanf("Filas saneadas: %d\n", r.RowsChanged)
	}
	for _, a := range r.Artifacts {
		if a.Status == domain.ArtifactPending {
			humanf("Artefacto pendiente: %s (%s)\n", a.Identity, a.ErrorCode)
		}
	}
	for _, p := range r.Pending {
		humanf("Pendiente %s: %s\n", p.Code, p.Action)
	}
	humanf("Copias externas: %s\n", r.ExternalCopies)
	if r.Mode == domain.SanitizeModePreview && len(rows) > 0 {
		humanln("Aplica con: mem sanitize --apply (detén antes los clientes de gomemory abiertos)")
	}
}

func summarizeIDs(ids []string) string {
	const shown = 5
	if len(ids) <= shown {
		return strings.Join(ids, ", ")
	}
	return strings.Join(ids[:shown], ", ") + fmt.Sprintf(", … (+%d)", len(ids)-shown)
}
