package persistence

import (
	"database/sql"
	"fmt"
	"strings"

	"mem/domain"
)

func FileHashesQuery(db *sql.DB, project string) (map[string]string, error) {
	rows, err := db.Query(`SELECT path, hash FROM code_files WHERE project = ?`, project)
	if err != nil {
		return nil, fmt.Errorf("file hashes: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, fmt.Errorf("scan file hash: %w", err)
		}
		out[path] = hash
	}
	return out, rows.Err()
}

// ReplaceFileNodes reemplaza atómicamente los nodos de un archivo: upsert de
// code_files (hash), borra los nodos viejos de ese archivo (y sus filas FTS),
// inserta los nuevos y los devuelve con IDs asignados.
func ReplaceFileNodes(db *sql.DB, project, path, hash string, nodes []domain.CodeNode) ([]domain.CodeNode, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("replace file nodes: %w", err)
	}
	defer tx.Rollback()

	var fileID int64
	err = tx.QueryRow(`SELECT id FROM code_files WHERE project = ? AND path = ?`, project, path).Scan(&fileID)
	if err == sql.ErrNoRows {
		res, err := tx.Exec(
			`INSERT INTO code_files (project, path, hash, indexed_at) VALUES (?, ?, ?, `+Now+`)`,
			project, path, hash,
		)
		if err != nil {
			return nil, fmt.Errorf("insert code_files: %w", err)
		}
		fileID, err = res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("insert code_files: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("lookup code_files: %w", err)
	} else {
		if _, err := tx.Exec(`UPDATE code_files SET hash = ?, indexed_at = `+Now+` WHERE id = ?`, hash, fileID); err != nil {
			return nil, fmt.Errorf("update code_files: %w", err)
		}
	}

	oldIDs, err := queryNodeIDsByFile(tx, fileID)
	if err != nil {
		return nil, err
	}
	if len(oldIDs) > 0 {
		if err := deleteNodesByIDs(tx, oldIDs); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(`DELETE FROM code_edges WHERE src_file_id = ?`, fileID); err != nil {
		return nil, fmt.Errorf("delete stale edges: %w", err)
	}

	inserted := make([]domain.CodeNode, 0, len(nodes))
	for _, n := range nodes {
		n.Project = project
		n.File = path
		id, err := insertNode(tx, fileID, n)
		if err != nil {
			return nil, err
		}
		n.ID = id
		inserted = append(inserted, n)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("replace file nodes: %w", err)
	}
	return inserted, nil
}

func insertNode(tx *sql.Tx, fileID int64, n domain.CodeNode) (int64, error) {
	exported := 0
	if n.Exported {
		exported = 1
	}
	res, err := tx.Exec(
		`INSERT INTO code_nodes (project, file_id, kind, name, package, file, receiver, signature, start_line, end_line, exported)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Project, fileID, string(n.Kind), n.Name, n.Package, n.File, n.Receiver, n.Signature, n.StartLine, n.EndLine, exported,
	)
	if err != nil {
		return 0, fmt.Errorf("insert code_nodes: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert code_nodes: %w", err)
	}
	tx.Exec(`INSERT INTO code_search (rowid, name, signature, package, node_id) VALUES (?, ?, ?, ?, ?)`,
		id, n.Name, n.Signature, n.Package, id)
	return id, nil
}

func queryNodeIDsByFile(tx *sql.Tx, fileID int64) ([]int64, error) {
	rows, err := tx.Query(`SELECT id FROM code_nodes WHERE file_id = ?`, fileID)
	if err != nil {
		return nil, fmt.Errorf("query node ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan node id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func deleteNodesByIDs(tx *sql.Tx, ids []int64) error {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := "(" + strings.Join(placeholders, ",") + ")"

	if _, err := tx.Exec(`DELETE FROM code_search WHERE node_id IN `+inClause, args...); err != nil {
		return fmt.Errorf("delete code_search: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM code_edges WHERE from_id IN `+inClause+` OR to_id IN `+inClause, append(append([]any{}, args...), args...)...); err != nil {
		return fmt.Errorf("delete stale node edges: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM code_nodes WHERE id IN `+inClause, args...); err != nil {
		return fmt.Errorf("delete code_nodes: %w", err)
	}
	return nil
}

func DeleteCodeFile(db *sql.DB, project, path string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("delete code file: %w", err)
	}
	defer tx.Rollback()

	var fileID int64
	err = tx.QueryRow(`SELECT id FROM code_files WHERE project = ? AND path = ?`, project, path).Scan(&fileID)
	if err == sql.ErrNoRows {
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("lookup code_files: %w", err)
	}

	ids, err := queryNodeIDsByFile(tx, fileID)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := deleteNodesByIDs(tx, ids); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM code_edges WHERE src_file_id = ?`, fileID); err != nil {
		return fmt.Errorf("delete stale edges: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM code_files WHERE id = ?`, fileID); err != nil {
		return fmt.Errorf("delete code_files: %w", err)
	}
	return tx.Commit()
}

// InsertCodeEdges reemplaza las aristas originadas en un archivo: borra las
// viejas de ese origen e inserta las nuevas en una transacción.
func InsertCodeEdges(db *sql.DB, project, srcPath string, edges []domain.CodeEdge) error {
	if len(edges) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("insert code edges: %w", err)
	}
	defer tx.Rollback()

	var srcFileID int64
	err = tx.QueryRow(`SELECT id FROM code_files WHERE project = ? AND path = ?`, project, srcPath).Scan(&srcFileID)
	if err != nil {
		return fmt.Errorf("lookup src file: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM code_edges WHERE src_file_id = ?`, srcFileID); err != nil {
		return fmt.Errorf("delete stale edges: %w", err)
	}

	for _, e := range edges {
		_, err := tx.Exec(
			`INSERT OR IGNORE INTO code_edges (project, from_id, to_id, kind, confidence, src_file_id)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			project, e.FromID, e.ToID, string(e.Kind), e.Confidence, srcFileID,
		)
		if err != nil {
			return fmt.Errorf("insert code_edges: %w", err)
		}
	}
	return tx.Commit()
}

func SearchCodeNodes(db *sql.DB, project, query string, limit int) ([]domain.CodeNode, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	if nodes, err := searchCodeNodesFTS(db, project, query, limit); err == nil {
		return nodes, nil
	}
	return searchCodeNodesLike(db, project, query, limit)
}

func searchCodeNodesFTS(db *sql.DB, project, query string, limit int) ([]domain.CodeNode, error) {
	ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	rows, err := db.Query(
		`SELECT n.id, n.project, n.kind, n.name, n.package, n.file, n.receiver, n.signature, n.start_line, n.end_line, n.exported
		 FROM code_search s
		 JOIN code_nodes n ON n.id = s.node_id
		 WHERE s.code_search MATCH ? AND n.project = ?
		 ORDER BY rank
		 LIMIT ?`,
		ftsQuery, project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCodeNodes(rows)
}

func searchCodeNodesLike(db *sql.DB, project, query string, limit int) ([]domain.CodeNode, error) {
	like := "%" + query + "%"
	rows, err := db.Query(
		`SELECT id, project, kind, name, package, file, receiver, signature, start_line, end_line, exported
		 FROM code_nodes
		 WHERE project = ? AND (name LIKE ? OR signature LIKE ? OR package LIKE ?)
		 ORDER BY
		   CASE WHEN name LIKE ? THEN 0 ELSE 1 END,
		   name
		 LIMIT ?`,
		project, like, like, like, like, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search code nodes (like): %w", err)
	}
	defer rows.Close()
	return scanCodeNodes(rows)
}

// UpsertPackageNode devuelve el nodo paquete (kind='package', file_id=0) para
// una ruta de import, creándolo si no existe. file_id=0 es un centinela
// seguro: los IDs de code_files/code_nodes reales empiezan en 1
// (AUTOINCREMENT), así que ReplaceFileNodes (que borra por file_id real)
// nunca toca estas filas al reindexar un archivo.
func UpsertPackageNode(db *sql.DB, project, importPath string) (domain.CodeNode, error) {
	var n domain.CodeNode
	err := db.QueryRow(
		`SELECT id, name FROM code_nodes WHERE project = ? AND kind = ? AND name = ? AND file_id = 0`,
		project, string(domain.NodePackage), importPath,
	).Scan(&n.ID, &n.Name)
	if err == nil {
		n.Project = project
		n.Kind = domain.NodePackage
		return n, nil
	}
	if err != sql.ErrNoRows {
		return domain.CodeNode{}, fmt.Errorf("lookup package node: %w", err)
	}

	res, err := db.Exec(
		`INSERT INTO code_nodes (project, file_id, kind, name) VALUES (?, 0, ?, ?)`,
		project, string(domain.NodePackage), importPath,
	)
	if err != nil {
		return domain.CodeNode{}, fmt.Errorf("insert package node: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.CodeNode{}, fmt.Errorf("insert package node: %w", err)
	}
	return domain.CodeNode{ID: id, Project: project, Kind: domain.NodePackage, Name: importPath}, nil
}

func NodesByName(db *sql.DB, project, name string) ([]domain.CodeNode, error) {
	rows, err := db.Query(
		`SELECT id, project, kind, name, package, file, receiver, signature, start_line, end_line, exported
		 FROM code_nodes WHERE project = ? AND name = ?`,
		project, name,
	)
	if err != nil {
		return nil, fmt.Errorf("nodes by name: %w", err)
	}
	defer rows.Close()
	return scanCodeNodes(rows)
}

func scanCodeNodes(rows *sql.Rows) ([]domain.CodeNode, error) {
	var nodes []domain.CodeNode
	for rows.Next() {
		var n domain.CodeNode
		var kind string
		var exported int
		err := rows.Scan(&n.ID, &n.Project, &kind, &n.Name, &n.Package, &n.File, &n.Receiver,
			&n.Signature, &n.StartLine, &n.EndLine, &exported)
		if err != nil {
			return nil, fmt.Errorf("scan code node: %w", err)
		}
		n.Kind = domain.CodeNodeKind(kind)
		n.Exported = exported != 0
		nodes = append(nodes, n)
	}
	if nodes == nil {
		nodes = []domain.CodeNode{}
	}
	return nodes, rows.Err()
}

// Neighbors hace BFS por code_edges desde nodeID hasta depth saltos,
// filtrando por tipo de arista (o todas si kind == "") y dirección.
func Neighbors(db *sql.DB, project string, nodeID int64, kind domain.CodeEdgeKind, direction string, depth int) ([]domain.CodeNode, []domain.CodeEdge, error) {
	if depth <= 0 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}

	visited := map[int64]bool{nodeID: true}
	frontier := []int64{nodeID}
	var allEdges []domain.CodeEdge
	nodeIDSet := map[int64]bool{}

	for d := 0; d < depth && len(frontier) > 0; d++ {
		edges, err := edgesForFrontier(db, project, frontier, kind, direction)
		if err != nil {
			return nil, nil, err
		}

		var next []int64
		for _, e := range edges {
			allEdges = append(allEdges, e)
			other := e.ToID
			if visited[e.ToID] {
				other = e.FromID
			}
			if !visited[other] {
				visited[other] = true
				nodeIDSet[other] = true
				next = append(next, other)
			}
		}
		frontier = next
	}

	if len(nodeIDSet) == 0 {
		return []domain.CodeNode{}, allEdges, nil
	}

	ids := make([]int64, 0, len(nodeIDSet))
	for id := range nodeIDSet {
		ids = append(ids, id)
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, project)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	rows, err := db.Query(
		`SELECT id, project, kind, name, package, file, receiver, signature, start_line, end_line, exported
		 FROM code_nodes WHERE project = ? AND id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("neighbors nodes: %w", err)
	}
	defer rows.Close()
	nodes, err := scanCodeNodes(rows)
	return nodes, allEdges, err
}

func edgesForFrontier(db *sql.DB, project string, frontier []int64, kind domain.CodeEdgeKind, direction string) ([]domain.CodeEdge, error) {
	placeholders := make([]string, len(frontier))
	args := make([]any, len(frontier))
	for i, id := range frontier {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := "(" + strings.Join(placeholders, ",") + ")"

	where := "project = ? AND "
	switch direction {
	case "in":
		where += "to_id IN " + inClause
	case "out":
		where += "from_id IN " + inClause
	default: // "both"
		where += "(from_id IN " + inClause + " OR to_id IN " + inClause + ")"
	}

	queryArgs := []any{project}
	queryArgs = append(queryArgs, args...)
	if direction != "in" && direction != "out" {
		queryArgs = append(queryArgs, args...)
	}

	if kind != "" {
		where += " AND kind = ?"
		queryArgs = append(queryArgs, string(kind))
	}

	rows, err := db.Query(
		`SELECT id, project, from_id, to_id, kind, confidence FROM code_edges WHERE `+where,
		queryArgs...,
	)
	if err != nil {
		return nil, fmt.Errorf("edges for frontier: %w", err)
	}
	defer rows.Close()

	var edges []domain.CodeEdge
	for rows.Next() {
		var e domain.CodeEdge
		var k string
		if err := rows.Scan(&e.ID, &e.Project, &e.FromID, &e.ToID, &k, &e.Confidence); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		e.Kind = domain.CodeEdgeKind(k)
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

func CodeGraphStatus(db *sql.DB, project string) (domain.GraphStatus, error) {
	var status domain.GraphStatus

	err := db.QueryRow(`SELECT COUNT(*) FROM code_files WHERE project = ?`, project).Scan(&status.Files)
	if err != nil {
		return status, fmt.Errorf("count files: %w", err)
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM code_nodes WHERE project = ?`, project).Scan(&status.Nodes)
	if err != nil {
		return status, fmt.Errorf("count nodes: %w", err)
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM code_edges WHERE project = ?`, project).Scan(&status.Edges)
	if err != nil {
		return status, fmt.Errorf("count edges: %w", err)
	}

	var lastIndexed sql.NullString
	db.QueryRow(`SELECT MAX(indexed_at) FROM code_files WHERE project = ?`, project).Scan(&lastIndexed)
	if lastIndexed.Valid {
		status.LastIndexedAt = lastIndexed.String
	}

	rows, err := db.Query(
		`SELECT package, COUNT(*) as n FROM code_nodes
		 WHERE project = ? AND package != '' AND kind IN ('function', 'method', 'type')
		 GROUP BY package ORDER BY n DESC LIMIT 5`,
		project,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ps domain.PackageStat
			if err := rows.Scan(&ps.Package, &ps.Symbols); err == nil {
				status.TopPackages = append(status.TopPackages, ps)
			}
		}
	}

	return status, nil
}
