package persistence

import (
	"database/sql"
	"fmt"
	"strings"

	"mem/domain"
)

// Llamadas sin resolver del grafo propio (HCD-041). El reindexado es
// incremental por hash: un archivo que llama a un símbolo todavía inexistente
// no se vuelve a indexar cuando el símbolo aparece en otro archivo. Guardar la
// llamada pendiente permite resolverla en ese momento sin reindexar al que
// llama. Las pendientes de un archivo se borran con sus nodos
// (ReplaceFileNodes, DeleteCodeFile).

func fileIDOf(q interface {
	QueryRow(string, ...any) *sql.Row
}, project, path string) (int64, error) {
	var id int64
	if err := q.QueryRow(`SELECT id FROM code_files WHERE project = ? AND path = ?`, project, path).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup code_files %s: %w", path, err)
	}
	return id, nil
}

// ReplacePendingCalls reemplaza las pendientes del archivo srcPath.
func ReplacePendingCalls(db *sql.DB, project, srcPath string, calls []domain.PendingCall) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("replace pending calls: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	fileID, err := fileIDOf(tx, project, srcPath)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM code_pending_calls WHERE src_file_id = ?`, fileID); err != nil {
		return fmt.Errorf("delete pending calls: %w", err)
	}
	for _, c := range calls {
		if _, err := tx.Exec(
			`INSERT INTO code_pending_calls (project, src_file_id, caller_id, caller_pkg, name, sel_alias, import_path)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			project, fileID, c.CallerID, c.CallerPkg, c.Name, c.SelAlias, c.ImportPath); err != nil {
			return fmt.Errorf("insert pending call: %w", err)
		}
	}
	return tx.Commit()
}

// PendingCallsByNames devuelve las pendientes del proyecto hacia esos nombres.
func PendingCallsByNames(db *sql.DB, project string, names []string) ([]domain.PendingCall, error) {
	if len(names) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(names)+1)
	args = append(args, project)
	for _, n := range names {
		args = append(args, n)
	}
	rows, err := db.Query(
		`SELECT p.id, f.path, p.caller_id, p.caller_pkg, p.name, p.sel_alias, p.import_path
		 FROM code_pending_calls p JOIN code_files f ON f.id = p.src_file_id
		 WHERE p.project = ? AND p.name IN (?`+strings.Repeat(",?", len(names)-1)+`)
		 ORDER BY p.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("pending calls by names: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.PendingCall
	for rows.Next() {
		var c domain.PendingCall
		if err := rows.Scan(&c.ID, &c.SrcPath, &c.CallerID, &c.CallerPkg, &c.Name, &c.SelAlias, &c.ImportPath); err != nil {
			return nil, fmt.Errorf("scan pending call: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ResolvePendingCalls añade las aristas resueltas como originadas en srcPath,
// sin borrar sus demás aristas (a diferencia de InsertCodeEdges), y retira
// las pendientes resueltas, todo en una transacción.
func ResolvePendingCalls(db *sql.DB, project, srcPath string, edges []domain.CodeEdge, pendingIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("resolve pending calls: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	fileID, err := fileIDOf(tx, project, srcPath)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO code_edges (project, from_id, to_id, kind, confidence, src_file_id)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			project, e.FromID, e.ToID, string(e.Kind), e.Confidence, fileID); err != nil {
			return fmt.Errorf("insert resolved edge: %w", err)
		}
	}
	for _, id := range pendingIDs {
		if _, err := tx.Exec(`DELETE FROM code_pending_calls WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete resolved pending call: %w", err)
		}
	}
	return tx.Commit()
}
