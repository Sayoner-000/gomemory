package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"mem/domain"
)

// OptimizeStorage ejecuta la detección y purga de relaciones y nodos/aristas huérfanas,
// consolidación de duplicados y optimización de índices SQLite y FTS5.
func OptimizeStorage(ctx context.Context, db *sql.DB, project string, dryRun bool) (domain.StorageOptimizationReport, error) {
	report := domain.StorageOptimizationReport{
		Project: project,
		DryRun:  dryRun,
	}

	// 1. Contar/detectar relaciones huérfanas
	orphanRelQuery := `SELECT COUNT(*) FROM memory_relations
		WHERE (memory_id_a NOT IN (SELECT id FROM memories) OR memory_id_b NOT IN (SELECT id FROM memories))`
	orphanRelArgs := []any{}
	if project != "" {
		orphanRelQuery += ` AND project = ?`
		orphanRelArgs = append(orphanRelArgs, project)
	}
	if err := db.QueryRowContext(ctx, orphanRelQuery, orphanRelArgs...).Scan(&report.OrphanRelationsCleaned); err != nil {
		return report, fmt.Errorf("contar relaciones huérfanas: %w", err)
	}

	// 2. Contar/detectar auto-relaciones inválidas
	selfRelQuery := `SELECT COUNT(*) FROM memory_relations WHERE memory_id_a = memory_id_b`
	selfRelArgs := []any{}
	if project != "" {
		selfRelQuery += ` AND project = ?`
		selfRelArgs = append(selfRelArgs, project)
	}
	if err := db.QueryRowContext(ctx, selfRelQuery, selfRelArgs...).Scan(&report.SelfRelationsCleaned); err != nil {
		return report, fmt.Errorf("contar auto-relaciones: %w", err)
	}

	// 3. Contar/detectar relaciones duplicadas (mismo par y tipo, excluyendo huérfanas y auto-relaciones)
	// Para cada grupo (project, memory_id_a, memory_id_b, relation) se conserva la de mayor confianza (y mayor id como desempate)
	dupQuery := `SELECT COUNT(*) FROM memory_relations r1
		WHERE r1.memory_id_a != r1.memory_id_b
		  AND r1.memory_id_a IN (SELECT id FROM memories)
		  AND r1.memory_id_b IN (SELECT id FROM memories)
		  AND EXISTS (
			SELECT 1 FROM memory_relations r2
			WHERE r2.project = r1.project
			  AND r2.memory_id_a = r1.memory_id_a
			  AND r2.memory_id_b = r1.memory_id_b
			  AND r2.relation = r1.relation
			  AND (r2.confidence > r1.confidence OR (r2.confidence = r1.confidence AND r2.id > r1.id))
		  )`
	dupArgs := []any{}
	if project != "" {
		dupQuery += ` AND r1.project = ?`
		dupArgs = append(dupArgs, project)
	}
	if err := db.QueryRowContext(ctx, dupQuery, dupArgs...).Scan(&report.DuplicateRelationsMerged); err != nil {
		return report, fmt.Errorf("contar relaciones duplicadas: %w", err)
	}

	// 4. Contar/detectar nodos huérfanos de código (file_id o file huérfano)
	orphanNodeQuery := `SELECT COUNT(*) FROM code_nodes
		WHERE ((file_id != 0 AND file_id NOT IN (SELECT id FROM code_files))
		   OR (file != '' AND file NOT IN (SELECT path FROM code_files WHERE project = code_nodes.project)))`
	orphanNodeArgs := []any{}
	if project != "" {
		orphanNodeQuery += ` AND project = ?`
		orphanNodeArgs = append(orphanNodeArgs, project)
	}
	if err := db.QueryRowContext(ctx, orphanNodeQuery, orphanNodeArgs...).Scan(&report.OrphanCodeNodesCleaned); err != nil {
		return report, fmt.Errorf("contar nodos de código huérfanos: %w", err)
	}

	// 5. Contar/detectar aristas huérfanas de código (from_id o to_id no existen o serán eliminados, o src_file_id no existe)
	orphanNodeSubquery := `SELECT id FROM code_nodes
		WHERE ((file_id != 0 AND file_id NOT IN (SELECT id FROM code_files))
		   OR (file != '' AND file NOT IN (SELECT path FROM code_files WHERE project = code_nodes.project)))`
	if project != "" {
		orphanNodeSubquery += ` AND project = ?`
	}

	orphanEdgeSubquery := fmt.Sprintf(`SELECT id FROM code_edges
		WHERE (from_id NOT IN (SELECT id FROM code_nodes)
		   OR from_id IN (%s)
		   OR to_id NOT IN (SELECT id FROM code_nodes)
		   OR to_id IN (%s)
		   OR (src_file_id != 0 AND src_file_id NOT IN (SELECT id FROM code_files)))`,
		orphanNodeSubquery, orphanNodeSubquery)
	orphanEdgeArgs := []any{}
	if project != "" {
		orphanEdgeArgs = append(orphanEdgeArgs, project, project)
		orphanEdgeSubquery += ` AND project = ?`
		orphanEdgeArgs = append(orphanEdgeArgs, project)
	}
	orphanEdgeQuery := fmt.Sprintf(`SELECT COUNT(*) FROM (%s)`, orphanEdgeSubquery)
	if err := db.QueryRowContext(ctx, orphanEdgeQuery, orphanEdgeArgs...).Scan(&report.OrphanCodeEdgesCleaned); err != nil {
		return report, fmt.Errorf("contar aristas de código huérfanas: %w", err)
	}

	// 6. Contar/detectar paquetes desreferenciados (nodos package sin aristas entrantes ni salientes tras descontar aristas huérfanas)
	orphanPkgQuery := fmt.Sprintf(`SELECT COUNT(*) FROM code_nodes
		WHERE kind = 'package'
		  AND id NOT IN (SELECT to_id FROM code_edges WHERE id NOT IN (%s))
		  AND id NOT IN (SELECT from_id FROM code_edges WHERE id NOT IN (%s))`,
		orphanEdgeSubquery, orphanEdgeSubquery)
	orphanPkgArgs := []any{}
	if project != "" {
		orphanPkgArgs = append(orphanPkgArgs, orphanEdgeArgs...)
		orphanPkgArgs = append(orphanPkgArgs, orphanEdgeArgs...)
		orphanPkgQuery += ` AND project = ?`
		orphanPkgArgs = append(orphanPkgArgs, project)
	}
	if err := db.QueryRowContext(ctx, orphanPkgQuery, orphanPkgArgs...).Scan(&report.UnreferencedPackagesCleaned); err != nil {
		return report, fmt.Errorf("contar paquetes desreferenciados: %w", err)
	}

	if dryRun {
		return report, nil
	}

	// Ejecución de mutaciones dentro de una transacción
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return report, fmt.Errorf("iniciar tx de optimización: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Eliminar auto-relaciones
	delSelf := `DELETE FROM memory_relations WHERE memory_id_a = memory_id_b`
	if project != "" {
		delSelf += ` AND project = ?`
	}
	resSelf, err := tx.ExecContext(ctx, delSelf, selfRelArgs...)
	if err != nil {
		return report, fmt.Errorf("eliminar auto-relaciones: %w", err)
	}
	if n, err := resSelf.RowsAffected(); err == nil {
		report.SelfRelationsCleaned = n
	}

	// Eliminar relaciones huérfanas
	delOrphanRel := `DELETE FROM memory_relations
		WHERE (memory_id_a NOT IN (SELECT id FROM memories) OR memory_id_b NOT IN (SELECT id FROM memories))`
	if project != "" {
		delOrphanRel += ` AND project = ?`
	}
	resRel, err := tx.ExecContext(ctx, delOrphanRel, orphanRelArgs...)
	if err != nil {
		return report, fmt.Errorf("eliminar relaciones huérfanas: %w", err)
	}
	if n, err := resRel.RowsAffected(); err == nil {
		report.OrphanRelationsCleaned = n
	}

	// Eliminar relaciones duplicadas (manteniendo la de mayor confianza / mayor id)
	delDup := `DELETE FROM memory_relations
		WHERE id IN (
			SELECT r1.id FROM memory_relations r1
			WHERE EXISTS (
				SELECT 1 FROM memory_relations r2
				WHERE r2.project = r1.project
				  AND r2.memory_id_a = r1.memory_id_a
				  AND r2.memory_id_b = r1.memory_id_b
				  AND r2.relation = r1.relation
				  AND (r2.confidence > r1.confidence OR (r2.confidence = r1.confidence AND r2.id > r1.id))
			)`
	if project != "" {
		delDup += ` AND r1.project = ?`
	}
	delDup += `)`
	resDup, err := tx.ExecContext(ctx, delDup, dupArgs...)
	if err != nil {
		return report, fmt.Errorf("eliminar relaciones duplicadas: %w", err)
	}
	if n, err := resDup.RowsAffected(); err == nil {
		report.DuplicateRelationsMerged = n
	}
	_, _ = tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_relations_pair ON memory_relations(project, memory_id_a, memory_id_b)`)

	// Eliminar nodos huérfanos de archivos eliminados PRIMERO
	delOrphanNodes := `DELETE FROM code_nodes
		WHERE ((file_id != 0 AND file_id NOT IN (SELECT id FROM code_files))
		   OR (file != '' AND file NOT IN (SELECT path FROM code_files WHERE project = code_nodes.project)))`
	if project != "" {
		delOrphanNodes += ` AND project = ?`
	}
	resNodes, err := tx.ExecContext(ctx, delOrphanNodes, orphanNodeArgs...)
	if err != nil {
		return report, fmt.Errorf("eliminar nodos huérfanos: %w", err)
	}
	if n, err := resNodes.RowsAffected(); err == nil {
		report.OrphanCodeNodesCleaned = n
	}

	// Eliminar aristas huérfanas DESPUÉS (así captura aristas cuyos nodos acaban de ser eliminados)
	delOrphanEdges := `DELETE FROM code_edges
		WHERE (from_id NOT IN (SELECT id FROM code_nodes)
		   OR to_id NOT IN (SELECT id FROM code_nodes)
		   OR (src_file_id != 0 AND src_file_id NOT IN (SELECT id FROM code_files)))`
	delOrphanEdgesArgs := []any{}
	if project != "" {
		delOrphanEdges += ` AND project = ?`
		delOrphanEdgesArgs = append(delOrphanEdgesArgs, project)
	}
	resEdges, err := tx.ExecContext(ctx, delOrphanEdges, delOrphanEdgesArgs...)
	if err != nil {
		return report, fmt.Errorf("eliminar aristas huérfanas: %w", err)
	}
	if n, err := resEdges.RowsAffected(); err == nil {
		report.OrphanCodeEdgesCleaned = n
	}

	// Eliminar paquetes desreferenciados
	delOrphanPkgs := `DELETE FROM code_nodes
		WHERE kind = 'package'
		  AND id NOT IN (SELECT to_id FROM code_edges)
		  AND id NOT IN (SELECT from_id FROM code_edges)`
	delOrphanPkgsArgs := []any{}
	if project != "" {
		delOrphanPkgs += ` AND project = ?`
		delOrphanPkgsArgs = append(delOrphanPkgsArgs, project)
	}
	resPkgs, err := tx.ExecContext(ctx, delOrphanPkgs, delOrphanPkgsArgs...)
	if err != nil {
		return report, fmt.Errorf("eliminar paquetes desreferenciados: %w", err)
	}
	if n, err := resPkgs.RowsAffected(); err == nil {
		report.UnreferencedPackagesCleaned = n
	}

	// Sincronizar y optimizar FTS5 si aplica
	_ = limpiarIndiceDeMemorias(tx)
	_, _ = tx.ExecContext(ctx, `DELETE FROM code_search WHERE node_id NOT IN (SELECT id FROM code_nodes)`)
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_search (memory_search) VALUES ('optimize')`); err == nil {
		report.FTSOptimized = true
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO code_search (code_search) VALUES ('optimize')`); err == nil {
		report.FTSOptimized = true
	}

	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("commit optimización: %w", err)
	}

	// Mantenimiento SQLite de bajo nivel
	_, _ = db.ExecContext(ctx, `PRAGMA analysis_limit=400; PRAGMA optimize;`)
	report.DatabaseOptimized = true

	return report, nil
}
