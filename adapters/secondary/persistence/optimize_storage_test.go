package persistence

import (
	"context"
	"testing"

	"mem/domain"
)

func TestOptimizeStorage_RelationsAndCodeGraph(t *testing.T) {
	db, dbPath := abrirBaseDeBorrado(t)
	repo := NewMaintenanceRepository(db, dbPath)
	ctx := context.Background()
	project := "proj"

	// 1. Preparar memorias y relaciones
	mem1 := insertarMemoria(t, db, domain.Learning, "Memoria 1", "Contenido 1")
	mem2 := insertarMemoria(t, db, domain.Learning, "Memoria 2", "Contenido 2")

	// Simular base donde duplicados impidieron o eliminaron el índice único
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_relations_pair`)

	// a) Válida
	_, err := db.Exec(`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		VALUES (?, ?, ?, 'relates_to', 0.8, 'razon 1', '2026-01-01')`, project, mem1, mem2)
	if err != nil {
		t.Fatal(err)
	}
	// b) Duplicada (mismo par y tipo, mayor confianza)
	_, err = db.Exec(`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		VALUES (?, ?, ?, 'relates_to', 0.95, 'razon 2 mejor', '2026-01-02')`, project, mem1, mem2)
	if err != nil {
		t.Fatal(err)
	}
	// c) Huérfana (extremo B inexistente)
	_, err = db.Exec(`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		VALUES (?, ?, 99999, 'relates_to', 0.5, 'huerfana B', '2026-01-01')`, project, mem1)
	if err != nil {
		t.Fatal(err)
	}
	// d) Huérfana (extremo A inexistente)
	_, err = db.Exec(`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		VALUES (?, 88888, ?, 'relates_to', 0.5, 'huerfana A', '2026-01-01')`, project, mem2)
	if err != nil {
		t.Fatal(err)
	}
	// e) Auto-relación (A=B)
	_, err = db.Exec(`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		VALUES (?, ?, ?, 'relates_to', 0.5, 'autoreferencial', '2026-01-01')`, project, mem1, mem1)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Preparar code graph con huérfanos
	// Archivo válido
	resFile, err := db.Exec(`INSERT INTO code_files (project, path, hash, indexed_at) VALUES (?, 'pkg/valid.go', 'hash1', '2026-01-01')`, project)
	if err != nil {
		t.Fatal(err)
	}
	validFileID, _ := resFile.LastInsertId()

	// Nodos válidos
	res1, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, ?, 'pkg/valid.go', 'FuncA', 'func')`, project, validFileID)
	if err != nil {
		t.Fatal(err)
	}
	node1, _ := res1.LastInsertId()
	res2, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, ?, 'pkg/valid.go', 'FuncB', 'func')`, project, validFileID)
	if err != nil {
		t.Fatal(err)
	}
	node2, _ := res2.LastInsertId()

	// Nodo huérfano (pertenece a un file_id que no existe en code_files)
	_, err = db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, 9999, 'pkg/deleted.go', 'FuncDeleted', 'func')`, project)
	if err != nil {
		t.Fatal(err)
	}

	// Nodo paquete desreferenciado (sin aristas)
	_, err = db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, 0, '', 'orphan/pkg', 'package')`, project)
	if err != nil {
		t.Fatal(err)
	}

	// Arista válida
	_, err = db.Exec(`INSERT INTO code_edges (project, from_id, to_id, kind, src_file_id)
		VALUES (?, ?, ?, 'calls', ?)`, project, node1, node2, validFileID)
	if err != nil {
		t.Fatal(err)
	}
	// Arista huérfana (to_id inexistente)
	_, err = db.Exec(`INSERT INTO code_edges (project, from_id, to_id, kind, src_file_id)
		VALUES (?, ?, 77777, 'calls', ?)`, project, node1, validFileID)
	if err != nil {
		t.Fatal(err)
	}
	// Arista huérfana (from_id inexistente)
	_, err = db.Exec(`INSERT INTO code_edges (project, from_id, to_id, kind, src_file_id)
		VALUES (?, 66666, ?, 'calls', ?)`, project, node2, validFileID)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Probar DryRun
	reportDry, err := repo.OptimizeStorage(ctx, project, true)
	if err != nil {
		t.Fatalf("dry run error: %v", err)
	}
	if !reportDry.DryRun {
		t.Errorf("se esperaba reportDry.DryRun = true")
	}
	if reportDry.OrphanRelationsCleaned != 2 {
		t.Errorf("reportDry.OrphanRelationsCleaned = %d, se esperaban 2", reportDry.OrphanRelationsCleaned)
	}
	if reportDry.DuplicateRelationsMerged != 1 {
		t.Errorf("reportDry.DuplicateRelationsMerged = %d, se esperaba 1", reportDry.DuplicateRelationsMerged)
	}
	if reportDry.SelfRelationsCleaned != 1 {
		t.Errorf("reportDry.SelfRelationsCleaned = %d, se esperaba 1", reportDry.SelfRelationsCleaned)
	}
	if reportDry.OrphanCodeNodesCleaned != 1 {
		t.Errorf("reportDry.OrphanCodeNodesCleaned = %d, se esperaba 1", reportDry.OrphanCodeNodesCleaned)
	}
	if reportDry.OrphanCodeEdgesCleaned != 2 {
		t.Errorf("reportDry.OrphanCodeEdgesCleaned = %d, se esperaban 2", reportDry.OrphanCodeEdgesCleaned)
	}
	if reportDry.UnreferencedPackagesCleaned != 1 {
		t.Errorf("reportDry.UnreferencedPackagesCleaned = %d, se esperaba 1", reportDry.UnreferencedPackagesCleaned)
	}

	// Verificar que con DryRun no se eliminó nada de la BD
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM memory_relations WHERE project = ?`, project).Scan(&count)
	if count != 5 {
		t.Errorf("con dryRun se alteraron las relaciones: count=%d, esperado 5", count)
	}

	// 4. Probar Ejecución Real (DryRun = false)
	reportReal, err := repo.OptimizeStorage(ctx, project, false)
	if err != nil {
		t.Fatalf("real run error: %v", err)
	}
	if reportReal.DryRun {
		t.Errorf("se esperaba reportReal.DryRun = false")
	}
	if reportReal.OrphanRelationsCleaned != 2 || reportReal.DuplicateRelationsMerged != 1 || reportReal.SelfRelationsCleaned != 1 {
		t.Errorf("conteo de relaciones saneadas erróneo: %+v", reportReal)
	}
	if reportReal.OrphanCodeNodesCleaned != 1 || reportReal.OrphanCodeEdgesCleaned != 2 || reportReal.UnreferencedPackagesCleaned != 1 {
		t.Errorf("conteo de grafo saneado erróneo: %+v", reportReal)
	}
	if !reportReal.FTSOptimized || !reportReal.DatabaseOptimized {
		t.Errorf("se esperaba optimización de FTS y SQLite: %+v", reportReal)
	}

	// Verificar que quedó exactamente 1 relación válida con la confianza más alta (0.95)
	var relCount int
	var conf float64
	err = db.QueryRow(`SELECT COUNT(*), MAX(confidence) FROM memory_relations WHERE project = ?`, project).Scan(&relCount, &conf)
	if err != nil || relCount != 1 || conf != 0.95 {
		t.Errorf("resultado relaciones en BD inválido: relCount=%d, conf=%f, err=%v", relCount, conf, err)
	}

	// Verificar que quedó exactamente 1 arista válida y 2 nodos válidos
	var edgeCount, nodeCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM code_edges WHERE project = ?`, project).Scan(&edgeCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM code_nodes WHERE project = ?`, project).Scan(&nodeCount)
	if edgeCount != 1 || nodeCount != 2 {
		t.Errorf("resultado code graph en BD inválido: edgeCount=%d, nodeCount=%d", edgeCount, nodeCount)
	}

	// 5. Idempotencia: segunda pasada no encuentra nada que limpiar
	reportIdempotent, err := repo.OptimizeStorage(ctx, project, false)
	if err != nil {
		t.Fatalf("idempotence error: %v", err)
	}
	if reportIdempotent.TotalCleaned() != 0 {
		t.Errorf("la segunda pasada debió encontrar 0 elementos a limpiar, got: %d", reportIdempotent.TotalCleaned())
	}
}

func TestOptimizeStorage_ProjectIsolationAndOrphanEdgesOrder(t *testing.T) {
	db, dbPath := abrirBaseDeBorrado(t)
	repo := NewMaintenanceRepository(db, dbPath)
	ctx := context.Background()
	projectTarget := "target_project"
	projectOther := "other_project"

	// C-001: Insertar nodo huérfano en projectOther
	_, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, 9999, 'missing.go', 'ForeignOrphan', 'func')`, projectOther)
	if err != nil {
		t.Fatal(err)
	}

	// C-002: En projectTarget, insertar un nodo válido, un nodo huérfano, y una arista entre ellos
	resFile, err := db.Exec(`INSERT INTO code_files (project, path, hash, indexed_at) VALUES (?, 'target/valid.go', 'hash1', '2026-11-04')`, projectTarget)
	if err != nil {
		t.Fatal(err)
	}
	validFileID, _ := resFile.LastInsertId()

	resValidNode, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, ?, 'target/valid.go', 'ValidNode', 'func')`, projectTarget, validFileID)
	if err != nil {
		t.Fatal(err)
	}
	validNodeID, _ := resValidNode.LastInsertId()

	// Nodo que referencia a un archivo inexistente en code_files
	resOrphanNode, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, ?, 'target/deleted.go', 'DeletedNode', 'func')`, projectTarget, validFileID)
	if err != nil {
		t.Fatal(err)
	}
	orphanNodeID, _ := resOrphanNode.LastInsertId()

	// Arista entre nodo válido y nodo que será purgado
	_, err = db.Exec(`INSERT INTO code_edges (project, from_id, to_id, kind, src_file_id) VALUES (?, ?, ?, 'calls', ?)`, projectTarget, validNodeID, orphanNodeID, validFileID)
	if err != nil {
		t.Fatal(err)
	}

	// Paquete referenciado únicamente por una arista cuyo extremo es el nodo huérfano
	resPkgNode, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, 0, '', 'target/orphanpkg', 'package')`, projectTarget)
	if err != nil {
		t.Fatal(err)
	}
	pkgNodeID, _ := resPkgNode.LastInsertId()
	_, err = db.Exec(`INSERT INTO code_edges (project, from_id, to_id, kind, src_file_id) VALUES (?, ?, ?, 'imports', ?)`, projectTarget, orphanNodeID, pkgNodeID, validFileID)
	if err != nil {
		t.Fatal(err)
	}

	// Dry run para projectTarget
	repDry, err := repo.OptimizeStorage(ctx, projectTarget, true)
	if err != nil {
		t.Fatalf("dry run error: %v", err)
	}
	if repDry.OrphanCodeNodesCleaned != 1 {
		t.Errorf("repDry.OrphanCodeNodesCleaned = %d, want 1", repDry.OrphanCodeNodesCleaned)
	}
	if repDry.OrphanCodeEdgesCleaned != 2 {
		t.Errorf("repDry.OrphanCodeEdgesCleaned = %d, want 2", repDry.OrphanCodeEdgesCleaned)
	}
	if repDry.UnreferencedPackagesCleaned != 1 {
		t.Errorf("repDry.UnreferencedPackagesCleaned = %d, want 1", repDry.UnreferencedPackagesCleaned)
	}

	// Ejecución real para projectTarget
	repReal, err := repo.OptimizeStorage(ctx, projectTarget, false)
	if err != nil {
		t.Fatalf("real run error: %v", err)
	}
	if repReal.OrphanCodeNodesCleaned != 1 {
		t.Errorf("repReal.OrphanCodeNodesCleaned = %d, want 1", repReal.OrphanCodeNodesCleaned)
	}
	if repReal.OrphanCodeEdgesCleaned != 2 {
		t.Errorf("repReal.OrphanCodeEdgesCleaned = %d, want 2", repReal.OrphanCodeEdgesCleaned)
	}
	if repReal.UnreferencedPackagesCleaned != 1 {
		t.Errorf("repReal.UnreferencedPackagesCleaned = %d, want 1", repReal.UnreferencedPackagesCleaned)
	}

	// C-001 Verificación: El nodo de projectOther debe permanecer intacto
	var otherCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM code_nodes WHERE project = ?`, projectOther).Scan(&otherCount); err != nil || otherCount != 1 {
		t.Errorf("C-001: el nodo de other_project no debió tocarse: count=%d, err=%v", otherCount, err)
	}

	// C-002 Verificación: Cero aristas colgantes en una sola pasada
	var danglingEdges int
	if err := db.QueryRow(`SELECT COUNT(*) FROM code_edges WHERE from_id NOT IN (SELECT id FROM code_nodes) OR to_id NOT IN (SELECT id FROM code_nodes)`).Scan(&danglingEdges); err != nil || danglingEdges != 0 {
		t.Errorf("C-002: quedaron aristas colgantes tras la primera pasada: %d, err=%v", danglingEdges, err)
	}

	// Segunda pasada: 0 saneamientos
	repSecond, err := repo.OptimizeStorage(ctx, projectTarget, false)
	if err != nil {
		t.Fatalf("second pass error: %v", err)
	}
	if repSecond.TotalCleaned() != 0 {
		t.Errorf("segunda pasada debió ser 0, got %d", repSecond.TotalCleaned())
	}
}

func TestOptimizeStorage_FTSCodeSearchCleanup(t *testing.T) {
	db, dbPath := abrirBaseDeBorrado(t)
	repo := NewMaintenanceRepository(db, dbPath)
	ctx := context.Background()
	project := "fts_proj"

	// Crear nodo huérfano con documento FTS asociado
	resNode, err := db.Exec(`INSERT INTO code_nodes (project, file_id, file, name, kind) VALUES (?, 9999, 'deleted.go', 'SynthFunc', 'func')`, project)
	if err != nil {
		t.Fatal(err)
	}
	nodeID, _ := resNode.LastInsertId()

	_, err = db.Exec(`INSERT INTO code_search (name, signature, package, node_id) VALUES ('SynthFunc', 'func SynthFunc()', 'pkg', ?)`, nodeID)
	if err != nil {
		t.Fatal(err)
	}

	// Documento FTS ya huérfano de antemano
	_, err = db.Exec(`INSERT INTO code_search (name, signature, package, node_id) VALUES ('OldOrphan', 'func OldOrphan()', 'pkg', 88888)`)
	if err != nil {
		t.Fatal(err)
	}

	// Ejecución de optimización
	rep, err := repo.OptimizeStorage(ctx, project, false)
	if err != nil {
		t.Fatalf("optimize storage error: %v", err)
	}
	if rep.OrphanCodeNodesCleaned != 1 {
		t.Errorf("OrphanCodeNodesCleaned = %d, want 1", rep.OrphanCodeNodesCleaned)
	}

	// Verificar que no queden documentos huérfanos en code_search
	var orphanFTSCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM code_search WHERE node_id NOT IN (SELECT id FROM code_nodes)`).Scan(&orphanFTSCount); err != nil || orphanFTSCount != 0 {
		t.Errorf("quedaron %d documentos huérfanos en code_search (err: %v)", orphanFTSCount, err)
	}
}
