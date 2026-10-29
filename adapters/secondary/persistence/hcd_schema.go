package persistence

import "fmt"

// hcdSchema es el catálogo HCD v2 (feature 045): unidad, evidencia y
// aprobación (approve, close o cancel). Las decisiones viven en memories, no
// en una tabla propia.
const hcdSchema = `
CREATE TABLE IF NOT EXISTS hcd_units (
	project TEXT NOT NULL,
	id TEXT NOT NULL,
	number INTEGER NOT NULL,
	title TEXT NOT NULL,
	slug TEXT NOT NULL,
	kind TEXT NOT NULL,
	lane TEXT NOT NULL,
	why TEXT NOT NULL,
	state TEXT NOT NULL DEFAULT 'active',
	active INTEGER NOT NULL DEFAULT 0,
	doc_path TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project, id),
	UNIQUE (project, number)
);
CREATE INDEX IF NOT EXISTS idx_hcd_units_active ON hcd_units(project, active);
CREATE TABLE IF NOT EXISTS hcd_evidence (
	id TEXT PRIMARY KEY,
	project TEXT NOT NULL,
	unit_id TEXT NOT NULL,
	tasks TEXT NOT NULL,
	kind TEXT NOT NULL,
	expect_fail INTEGER NOT NULL DEFAULT 0,
	command TEXT NOT NULL DEFAULT '',
	exit_code INTEGER NOT NULL DEFAULT 0,
	excerpt TEXT NOT NULL DEFAULT '',
	revision TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_hcd_evidence_unit ON hcd_evidence(project, unit_id, created_at);
CREATE TABLE IF NOT EXISTS hcd_approvals (
	id TEXT PRIMARY KEY,
	project TEXT NOT NULL,
	unit_id TEXT NOT NULL,
	action TEXT NOT NULL,
	revision TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	origin TEXT NOT NULL,
	client TEXT NOT NULL,
	quote TEXT NOT NULL,
	created_at TEXT NOT NULL
);
`

// addHCDNormColumns (v6) guarda en cada aprobación las huellas de la
// Constitución y del Playbook vigentes. Va siempre por ALTER aditivo para que
// una base nueva y una migrada desde v5 tengan el mismo catálogo.
func addHCDNormColumns(ex schemaExecer) error {
	if err := addColumnIfMissing(ex, "hcd_approvals", "constitution_rev", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return addColumnIfMissing(ex, "hcd_approvals", "playbook_rev", "TEXT NOT NULL DEFAULT ''")
}

// hcdV1Tables es el catálogo HCD v1, que solo existió en el schema v4 y nunca
// se publicó en una release. ADR 0003: única excepción a «nunca DROP».
var hcdV1Tables = []string{
	"hcd_approval_prompts", "hcd_approvals", "hcd_blocks", "hcd_clarifications",
	"hcd_decisions", "hcd_evidence", "hcd_exceptions", "hcd_executions",
	"hcd_focus", "hcd_heartbeats", "hcd_links", "hcd_migrations",
	"hcd_normative_revisions", "hcd_operations", "hcd_playbook_adoptions", "hcd_units",
}

// dropHCDV1 retira el catálogo v1 de una base que viene de v4, dentro de la
// misma transacción que crea el catálogo v2. No toca ninguna otra tabla.
func dropHCDV1(ex schemaExecer, from int) error {
	if from != 4 {
		return nil
	}
	for _, table := range hcdV1Tables {
		// table sale de una lista constante; DROP no admite parámetros.
		if _, err := ex.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("drop hcd v1 %s: %w", table, err)
		}
	}
	return nil
}
