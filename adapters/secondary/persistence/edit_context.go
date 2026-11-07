package persistence

import (
	"database/sql"
	"fmt"
	"path"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// ListMemoriesByFilepath devuelve las memorias del proyecto ancladas a alguna
// de las rutas, de la más reciente a la más antigua (HCD-028). Se aceptan
// varias formas de la misma ruta (relativa y absoluta) porque las memorias
// antiguas guardaron una u otra. Incluye las fichas del historial enlazadas a
// la ruta por memory_files (HCD-046).
func ListMemoriesByFilepath(db *sql.DB, project string, paths []string, limit int) ([]domain.Memory, error) {
	if len(paths) == 0 {
		return []domain.Memory{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(paths)), ",")
	args := []any{project}
	for _, p := range paths {
		args = append(args, p)
	}
	args = append(args, project)
	for _, p := range paths {
		args = append(args, p)
	}
	args = append(args, limit)
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), COALESCE(origin_prompt,''), COALESCE(topic_key,''), COALESCE(source_review_id,''),
		        created_at, updated_at
		 FROM memories WHERE project = ? AND (filepath IN (`+placeholders+`)
		   OR id IN (SELECT memory_id FROM memory_files WHERE project = ? AND path IN (`+placeholders+`)))
		 ORDER BY created_at DESC, id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("memorias por archivo: %w", err)
	}
	defer func() { _ = rows.Close() }()
	mems, err := scanMemories(rows)
	if mems == nil {
		mems = []domain.Memory{}
	}
	return mems, err
}

// TestsCovering devuelve las funciones Test* de archivos _test.go que llaman
// directamente a algún símbolo definido en path, según el grafo propio
// (HCD-028). Dir es el paquete donde corren, listo para `go test <Dir>`.
func TestsCovering(db *sql.DB, project, filePath string, limit int) ([]domain.EditContextTest, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	rows, err := db.Query(
		`SELECT DISTINCT caller.name, caller.file
		 FROM code_nodes target
		 JOIN code_edges e ON e.to_id = target.id AND e.kind = ? AND e.project = target.project
		 JOIN code_nodes caller ON caller.id = e.from_id AND caller.project = target.project
		 WHERE target.project = ? AND target.file = ?
		   AND caller.name LIKE 'Test%' AND caller.file LIKE '%\_test.go' ESCAPE '\'
		 ORDER BY caller.name LIMIT ?`,
		string(domain.EdgeCalls), project, filePath, limit)
	if err != nil {
		return nil, fmt.Errorf("pruebas que cubren %s: %w", filePath, err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.EditContextTest
	for rows.Next() {
		var name, file string
		if err := rows.Scan(&name, &file); err != nil {
			return nil, fmt.Errorf("scan prueba: %w", err)
		}
		out = append(out, domain.EditContextTest{Name: name, Dir: "./" + path.Dir(file)})
	}
	return out, rows.Err()
}

// EditContextRepository adapta las dos consultas al puerto.
type EditContextRepository struct{ db *sql.DB }

func NewEditContextRepository(db *sql.DB) *EditContextRepository {
	return &EditContextRepository{db: db}
}

func (r *EditContextRepository) MemoriesByFilepath(project string, paths []string, limit int) ([]domain.Memory, error) {
	return ListMemoriesByFilepath(r.db, project, paths, limit)
}

func (r *EditContextRepository) TestsCovering(project, filePath string, limit int) ([]domain.EditContextTest, error) {
	return TestsCovering(r.db, project, filePath, limit)
}

var _ ports.EditContextSource = (*EditContextRepository)(nil)
