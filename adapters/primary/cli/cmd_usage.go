package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"mem/application/usecases"
	"mem/domain"
)

// contractVersion es la versión del contrato de contracts/usage-report.md
// (feature 020). Sube solo ante un cambio incompatible; añadir claves nuevas
// no la sube.
const contractVersion = 1

// CmdUsage implementa `mem usage`: reporte medido de cuánto ahorró gomemory
// al emitir contexto (feature 020). NO renombrar la función Usage() de
// cli.go — es el texto de ayuda, un concepto distinto.
func CmdUsage(deps *Deps, args []string) {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	session := fs.String("session", "", "Sesión concreta (default: la activa, o la más reciente con registros)")
	all := fs.Bool("all", false, "Todas las sesiones del proyecto")
	asJSON := fs.Bool("json", false, "Salida legible por máquina (contracts/usage-report.md)")
	if err := fs.Parse(args); err != nil {
		return
	}

	if deps.UsageRepo == nil {
		fail("el registro de uso no está disponible en este proyecto")
	}

	scope, sessionID := resolveUsageScope(deps, *session, *all)

	windowTokens := 0
	if deps.SettingsRepo != nil {
		windowTokens = deps.SettingsRepo.Read(deps.Root).UsageWindowTokens
	}

	// scope=="empty" NO es "sessionID vacío" — ese vacío significa "todas las
	// sesiones" para BuildUsageReport (ámbito --all). Sin actividad conocida
	// el reporte debe quedar en ceros, sin ir a buscar el histórico completo
	// del proyecto (bug encontrado en verificación en vivo, regla de campo 2).
	var report domain.UsageReport
	var err error
	if scope == usageScopeEmpty {
		report = domain.UsageReport{Project: deps.Project, WindowTokens: windowTokens}
	} else {
		report, err = usecases.BuildUsageReport(deps.UsageRepo, deps.Project, sessionID, windowTokens)
		if err != nil {
			fail("construir reporte de uso: %v", err)
		}
	}

	if deps.TokenCounter != nil {
		if schemaTokens, schemaOps, err := measurePublishedSchemas(deps, deps.Root, deps.Project, deps.TokenCounter); err == nil {
			report.SchemaTokens = schemaTokens
			report.SchemaOperations = schemaOps
		}
		// best-effort: si la medición de esquemas falla, el reporte se
		// entrega igual, solo sin esa sección (nunca bloquea `mem usage`).
	}

	if *asJSON {
		data, err := json.MarshalIndent(newUsageReportJSON(report, scope), "", "  ")
		if err != nil {
			fail("serializar reporte de uso: %v", err)
		}
		os.Stdout.Write(data)
		os.Stdout.WriteString("\n")
		return
	}

	fmt.Print(usecases.FormatUsageReport(report, string(scope)))
}

// usageScope identifica cómo se resolvió la sesión del reporte (FR-010).
type usageScope string

const (
	usageScopeSession usageScope = "session"
	usageScopeAll     usageScope = "all"
	usageScopeEmpty   usageScope = "empty"
)

// resolveUsageScope decide el ámbito del reporte: --session explícito, --all,
// o por defecto la sesión activa; si no hay ninguna activa, la más reciente
// con registros; si tampoco hay ninguna, el ámbito queda "empty" (nunca un
// error — FR-010, edge case de la spec: "sin sesión activa").
func resolveUsageScope(deps *Deps, explicitSession string, all bool) (usageScope, string) {
	if all {
		return usageScopeAll, ""
	}
	if explicitSession != "" {
		return usageScopeSession, explicitSession
	}
	if deps.SessionRepo != nil {
		if active, _ := deps.SessionRepo.Active(deps.Project); active != nil {
			return usageScopeSession, active.ID
		}
	}
	if sessions, err := deps.UsageRepo.Sessions(deps.Project, 1); err == nil && len(sessions) > 0 {
		return usageScopeSession, sessions[0]
	}
	return usageScopeEmpty, ""
}

// usageBucketJSON/usageReportJSON son la forma legible por máquina exacta de
// contracts/usage-report.md — la forma que manda para cualquier consumidor
// (FR-012).
type usageBucketJSON struct {
	Key            string `json:"key"`
	Calls          int    `json:"calls"`
	BaselineTokens int    `json:"baseline_tokens"`
	EmittedTokens  int    `json:"emitted_tokens"`
}

type usageReportJSON struct {
	ContractVersion  int               `json:"contract_version"`
	Project          string            `json:"project"`
	Scope            string            `json:"scope"`
	SessionID        string            `json:"session_id,omitempty"`
	CountingMethod   string            `json:"counting_method"`
	CountingNote     string            `json:"counting_note"`
	Calls            int               `json:"calls"`
	BaselineTokens   int               `json:"baseline_tokens"`
	EmittedTokens    int               `json:"emitted_tokens"`
	SavedTokens      int               `json:"saved_tokens"`
	ReductionRatio   float64           `json:"reduction_ratio"`
	SchemaTokens     int               `json:"schema_tokens"`
	SchemaOperations int               `json:"schema_operations"`
	WindowTokens     int               `json:"window_tokens"`
	WindowRatio      *float64          `json:"window_ratio"`
	ByOperation      []usageBucketJSON `json:"by_operation"`
	ByChannel        []usageBucketJSON `json:"by_channel"`
}

func newUsageReportJSON(report domain.UsageReport, scope usageScope) usageReportJSON {
	out := usageReportJSON{
		ContractVersion:  contractVersion,
		Project:          report.Project,
		Scope:            string(scope),
		SessionID:        report.SessionID,
		CountingMethod:   "approximate",
		CountingNote:     "Aproximación neutral (~4 caracteres por token). Comparable contra sí misma.",
		Calls:            report.Calls,
		BaselineTokens:   report.BaselineTokens,
		EmittedTokens:    report.EmittedTokens,
		SavedTokens:      report.Saved(),
		ReductionRatio:   report.ReductionRatio(),
		SchemaTokens:     report.SchemaTokens,
		SchemaOperations: report.SchemaOperations,
		WindowTokens:     report.WindowTokens,
		ByOperation:      toBucketJSON(report.ByOperation),
		ByChannel:        toBucketJSON(report.ByChannel),
	}
	if ratio, ok := report.WindowRatio(); ok {
		out.WindowRatio = &ratio
	}
	return out
}

func toBucketJSON(buckets []domain.UsageBucket) []usageBucketJSON {
	out := make([]usageBucketJSON, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, usageBucketJSON{Key: b.Key, Calls: b.Calls, BaselineTokens: b.BaselineTokens, EmittedTokens: b.EmittedTokens})
	}
	return out
}
