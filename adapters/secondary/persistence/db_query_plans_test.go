package persistence

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryPlans_HotQueriesNeverScan(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	queries := []struct{ name, sql string }{
		// channel_activity.go:102
		{"session count", `SELECT COUNT(*) FROM sessions WHERE project = ? AND created_at >= ?`},
		// session.go:102
		{"active session", `SELECT id, project, COALESCE(summary,''), created_at, ended_at FROM sessions WHERE project = ? AND ended_at IS NULL ORDER BY created_at DESC, rowid DESC LIMIT 1`},
		// session.go:126
		{"recent sessions", `SELECT id, project, COALESCE(summary,''), created_at, COALESCE(ended_at,'') FROM sessions WHERE project = ? ORDER BY created_at DESC, rowid DESC LIMIT ?`},
		// memory.go:508
		{"last prompt", `SELECT last_prompt FROM sessions WHERE project = ? AND ended_at IS NULL ORDER BY created_at DESC, rowid DESC LIMIT 1`},
		// memory.go:381
		{"topic dedup", `SELECT id FROM memories WHERE project = ? AND topic_key = ? ORDER BY id DESC LIMIT 1`},
		// memory.go:417
		{"title dedup", `SELECT id FROM memories WHERE project = ? AND type = ? AND title = ? AND julianday(` + Now + `) - julianday(created_at) <= ? ORDER BY id DESC LIMIT 1`},
		// memory.go:461
		{"session anchor", `SELECT id FROM memories WHERE project = ? AND session_id = ? AND id <> ? AND type <> ? ORDER BY id DESC LIMIT 1`},
		// memory.go:611
		{"recent memories", `SELECT id FROM memories WHERE project = ? ORDER BY created_at DESC, id DESC LIMIT ?`},
		// code_graph.go:12
		{"code files", `SELECT path, hash FROM code_files WHERE project = ?`},
		// code_graph.go:307
		{"code nodes", `SELECT id, project, kind, name, package, file, receiver, signature, start_line, end_line, exported FROM code_nodes WHERE project = ? AND name = ?`},
		// code_graph.go:70
		{"code edges", `DELETE FROM code_edges WHERE src_file_id = ?`},
		// review.go:1072
		{"findings", `SELECT id, reviewer_result_id, local_id, location, severity, category, claim, evidence_class, evidence, confidence FROM findings WHERE reviewer_result_id = ? ORDER BY id`},
		// review.go:1471
		{"fix rounds", `SELECT COUNT(*) FROM fix_rounds WHERE review_id = ?`},
		// review.go:452
		{"review status", `SELECT status FROM reviewer_results WHERE review_id = ? AND reviewer = ? AND round = ?`},
		// context_delivery.go:48
		{"context deliveries", `SELECT content_hash FROM context_deliveries WHERE session_id = ? AND kind = ?`},
		// delivered_blocks.go:50
		{"delivered blocks", `SELECT block_hash FROM delivered_blocks WHERE session_id = ? AND block_hash IN (?)`},
		// usage.go:48
		{"usage", `SELECT session_id FROM usage_records WHERE project = ? ORDER BY id ASC`},
		// channel_activity.go:180
		{"guard events", `SELECT agent, kind, SUM(count),
                (SELECT g2.last_detail FROM hook_guard_events g2
                  WHERE g2.project = g.project AND g2.agent = g.agent AND g2.kind = g.kind
                  ORDER BY g2.updated_at DESC LIMIT 1)
           FROM hook_guard_events g
          WHERE project = ? AND day >= date(` + Now + `, ?)
          GROUP BY agent, kind
          ORDER BY agent, kind`},
		// instincts.go:89
		{"instincts", `SELECT slug,kind,description,observations,corrections,confidence,scope,status FROM instinct_stats WHERE project=? AND (? OR status!='rejected') ORDER BY confidence DESC,slug`},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			args := make([]any, strings.Count(query.sql, "?"))
			for i := range args {
				args[i] = 1
			}
			rows, err := db.Query("EXPLAIN QUERY PLAN "+query.sql, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			found := false
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				found = true
				fields := strings.Fields(detail)
				if len(fields) >= 2 && fields[0] == "SCAN" && fields[1] != "code_search" && fields[1] != "memory_search" {
					t.Errorf("hot query scans a table: %s", detail)
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("query plan returned no rows")
			}
		})
	}
}
