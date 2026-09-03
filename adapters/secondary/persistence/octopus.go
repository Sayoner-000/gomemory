package persistence

import (
	"database/sql"

	"mem/domain"
)

// OctopusRepository persiste decisiones de enrutamiento y reportes de ejecución
// (feature 027).
//
// Fire-and-forget: ningún método devuelve error. Medir no puede impedir enrutar
// ni ejecutar — mismo criterio que UsageRepository. Un fallo de escritura se
// traga en silencio a propósito: la alternativa sería que una base bloqueada
// tumbara una decisión de enrutamiento, que es exactamente lo contrario de lo
// que la telemetría debe hacer por el usuario.
type OctopusRepository struct {
	db *sql.DB
}

func NewOctopusRepository(db *sql.DB) *OctopusRepository {
	return &OctopusRepository{db: db}
}

// RecordDecision guarda una decisión recién emitida.
func (r *OctopusRepository) RecordDecision(project, planID string, class domain.TaskClass, d domain.RouteDecision) {
	if r == nil || r.db == nil || project == "" || d.WorkUnitID == "" {
		return
	}
	estimated := 0
	if d.Route.Delegada() {
		estimated = d.EstimatedCost.Total()
	}
	r.db.Exec(`
		INSERT INTO octopus_executions
			(project, plan_id, task_id, task_class, route, reason_code,
			 parallel_group, context_budget, output_budget, estimated_tokens, decided_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, `+Now+`)`,
		project, planID, d.WorkUnitID, string(class), string(d.Route), string(d.Reason),
		d.ParallelGroup, d.ContextBudget, d.OutputBudget, estimated)
}

// RecordReport completa la fila de la decisión más reciente de esa tarea.
//
// Un reporte para una tarea sin decisión previa NO es un error: el UPDATE no
// afecta a ninguna fila y ahí acaba. El runtime nunca debe romperse por
// informarnos de algo que no le pedimos.
func (r *OctopusRepository) RecordReport(project string, rep domain.ExecutionReport) {
	if r == nil || r.db == nil || project == "" || rep.TaskID == "" {
		return
	}
	r.db.Exec(`
		UPDATE octopus_executions
		SET route = CASE WHEN ? <> '' THEN ? ELSE route END,
		    status = ?, context_tokens = ?, output_tokens = ?,
		    duration_ms = ?, quality = ?, reported_at = `+Now+`
		WHERE id = (
			SELECT id FROM octopus_executions
			WHERE project = ? AND task_id = ? AND reported_at IS NULL
			ORDER BY id DESC LIMIT 1
		)`,
		string(rep.Route), string(rep.Route), string(rep.Status), rep.ContextTokens, rep.OutputTokens,
		rep.DurationMS, string(rep.Quality), project, rep.TaskID)
}

// Evidence agrega el historial por clase de tarea. Un proyecto sin historial
// devuelve un mapa vacío: el arranque en frío es el caso normal el primer día.
func (r *OctopusRepository) Evidence(project string) map[domain.TaskClass]*domain.ClassEvidence {
	out := map[domain.TaskClass]*domain.ClassEvidence{}
	if r == nil || r.db == nil || project == "" {
		return out
	}

	rows, err := r.db.Query(`
		SELECT task_class,
		       COUNT(*),
		       COALESCE(AVG(CASE WHEN route = 'INLINE' THEN context_tokens + output_tokens END), 0),
		       COALESCE(AVG(CASE WHEN route IN ('DELEGATE','PARALLEL') THEN context_tokens + output_tokens END), 0),
		       COALESCE(AVG(CASE WHEN route IN ('DELEGATE','PARALLEL') THEN context_tokens END), 0),
		       COALESCE(AVG(CASE WHEN route IN ('DELEGATE','PARALLEL') THEN CASE WHEN status = 'completed' THEN 1.0 ELSE 0.0 END END), 0)
		FROM octopus_executions
		WHERE project = ? AND task_class <> '' AND reported_at IS NOT NULL
		GROUP BY task_class`, project)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var clase string
		var e domain.ClassEvidence
		var inlineAvg, delegAvg, ctxAvg float64
		if err := rows.Scan(&clase, &e.Executions, &inlineAvg, &delegAvg, &ctxAvg, &e.SuccessRate); err != nil {
			continue
		}
		e.Class = domain.TaskClass(clase)
		e.InlineAvgTokens = int(inlineAvg)
		e.DelegatedAvgTokens = int(delegAvg)
		e.DelegatedAvgContextTokens = int(ctxAvg)
		out[e.Class] = &e
	}
	return out
}

// Stats agrega la telemetría del proyecto.
func (r *OctopusRepository) Stats(project string) domain.RoutingStats {
	stats := domain.RoutingStats{PorRuta: map[domain.Route]int{}}
	if r == nil || r.db == nil || project == "" {
		return stats
	}

	rows, err := r.db.Query(`
		SELECT route, COUNT(*), COALESCE(SUM(CASE WHEN reported_at IS NOT NULL THEN estimated_tokens ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN reported_at IS NOT NULL THEN COALESCE(context_tokens,0) + COALESCE(output_tokens,0) ELSE 0 END), 0),
		       SUM(CASE WHEN reported_at IS NOT NULL THEN 1 ELSE 0 END),
		       SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN status = 'insufficient_context' THEN 1 ELSE 0 END)
		FROM octopus_executions WHERE project = ? GROUP BY route`, project)
	if err != nil {
		return stats
	}
	defer rows.Close()

	for rows.Next() {
		var route string
		var n, estimados, reales, conReporte, exitos, fallos, insuf int
		if err := rows.Scan(&route, &n, &estimados, &reales, &conReporte, &exitos, &fallos, &insuf); err != nil {
			continue
		}
		stats.PorRuta[domain.Route(route)] = n
		stats.Decisiones += n
		stats.TokensEstimados += estimados
		stats.TokensReales += reales
		stats.ConReporte += conReporte
		stats.Exitos += exitos
		stats.Fallos += fallos
		stats.ContextoInsuf += insuf
	}

	// Ancho de paralelismo: el grupo más grande observado.
	r.db.QueryRow(`
		SELECT COALESCE(MAX(n), 0) FROM (
			SELECT COUNT(*) AS n FROM octopus_executions
			WHERE project = ? AND parallel_group <> ''
			GROUP BY plan_id, parallel_group, decided_at
		)`, project).Scan(&stats.AnchoParaleloMax)

	return stats
}

// History devuelve las últimas decisiones, con su reporte si llegó.
func (r *OctopusRepository) History(project string, class domain.TaskClass, limit int) []domain.ExecutionRecord {
	if r == nil || r.db == nil || project == "" {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}

	consulta := `
		SELECT task_id, plan_id, task_class, route, reason_code,
		       context_budget, output_budget, estimated_tokens, decided_at,
		       COALESCE(status,''), COALESCE(context_tokens,0), COALESCE(output_tokens,0),
		       COALESCE(duration_ms,0), COALESCE(quality,''), COALESCE(reported_at,'')
		FROM octopus_executions
		WHERE project = ?`
	args := []any{project}
	if class != "" {
		consulta += ` AND task_class = ?`
		args = append(args, string(class))
	}
	consulta += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.Query(consulta, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []domain.ExecutionRecord
	for rows.Next() {
		var rec domain.ExecutionRecord
		var clase, ruta, razon, estado, calidad string
		if err := rows.Scan(&rec.TaskID, &rec.PlanID, &clase, &ruta, &razon,
			&rec.ContextBudget, &rec.OutputBudget, &rec.EstimatedCost, &rec.DecidedAt,
			&estado, &rec.ContextTokens, &rec.OutputTokens, &rec.DurationMS,
			&calidad, &rec.ReportedAt); err != nil {
			continue
		}
		rec.Class = domain.TaskClass(clase)
		rec.Route = domain.Route(ruta)
		rec.Reason = domain.Reason(razon)
		rec.Status = domain.ResultStatus(estado)
		rec.Quality = domain.Quality(calidad)
		rec.Reported = rec.ReportedAt != ""
		out = append(out, rec)
	}
	return out
}
