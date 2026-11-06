package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mem/adapters/secondary/gitdiff"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// HCD-032: dos revisiones ACR encadenadas sobre un repositorio git real. La
// segunda hereda lo asentado, no re-adjudica lo que no cambió y reabre lo que
// cambió.
func TestReviewAcumulativa(t *testing.T) {
	// Almacén aislado: sin esto la prueba escribía en el store real del usuario,
	// y el binario de HCD-035 necesita leer la misma base.
	data := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", data)
	dir := t.TempDir()
	gitc := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitc("init", "-q")
	// rules.go pesa más que la lista de heredados: así el ahorro de HCD-035 es
	// positivo cuando solo cambia x.go.
	write("domain/rules.go", "package domain\n"+strings.Repeat("// regla del dominio\n", 400))
	write("adapters/x.go", "package adapters\n")
	gitc("add", ".")
	gitc("commit", "-q", "-m", "uno")
	commit1 := gitc("rev-parse", "HEAD")

	db, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	reviews, ledger := persistence.NewReviewRepository(db), persistence.NewConsensusRepository(db)
	project := persistence.ProjectKey(dir)
	f := func(local, loc string, ref int64) domain.Finding {
		return domain.Finding{LocalID: local, Location: loc, Severity: domain.SeverityHigh, Category: "c", Claim: "sospecha en " + loc,
			EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"e"}, Confidence: "medium", SettledRef: ref}
	}
	submit := func(r *domain.Review, reviewer domain.Reviewer, fs ...domain.Finding) {
		t.Helper()
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{Project: project, ReviewID: r.ID,
			TargetDigest: r.ActiveTargetDigest(),
			Result:       domain.ReviewerResult{Reviewer: reviewer, Status: domain.ReviewerResultSuccess, Findings: fs}}); err != nil {
			t.Fatal(err)
		}
	}
	findingIDs := func(r *domain.Review) []domain.Finding {
		results, _ := reviews.ListReviewerResults(project, r.ID, 0)
		var out []domain.Finding
		for _, res := range results {
			out = append(out, res.Findings...)
		}
		return out
	}

	// Revisión 1: dos SUSPECT, uno por archivo; aprueba.
	r1, err := usecases.StartReview(reviews, usecases.StartReviewInput{Project: project, TargetType: domain.TargetCommit, Revision: commit1, Digest: commit1})
	if err != nil {
		t.Fatal(err)
	}
	submit(r1, domain.ReviewerA, f("A-1", "domain/rules.go:1", 0), f("A-2", "adapters/x.go:1", 0))
	submit(r1, domain.ReviewerB)
	var unmatched []usecases.ConsensusUnmatched
	for _, fd := range findingIDs(r1) {
		unmatched = append(unmatched, usecases.ConsensusUnmatched{FindingID: fd.ID, Status: domain.ConsensusSuspect, Severity: domain.SeverityHigh})
	}
	if _, err := usecases.BuildConsensus(reviews, ledger, usecases.BuildConsensusInput{Project: project, ReviewID: r1.ID, Unmatched: unmatched}); err != nil {
		t.Fatal(err)
	}
	if done, err := usecases.FinalizeReview(reviews, ledger, project, r1.ID); err != nil || done.Verdict != domain.VerdictApproved {
		t.Fatalf("la revisión 1 aprueba con SUSPECT: %+v %v", done, err)
	}

	// Commit 2: solo cambia adapters/x.go.
	write("adapters/x.go", "package adapters\n// cambio\n")
	gitc("commit", "-qam", "dos")
	commit2 := gitc("rev-parse", "HEAD")

	// Revisión 2 enlazada: hereda los dos SUSPECT y el delta es adapters/x.go.
	r2, err := usecases.StartReviewSince(reviews, ledger, gitdiff.New(dir), usecases.StartReviewInput{Project: project,
		TargetType: domain.TargetCommit, Revision: commit2, Digest: commit2, SinceReviewID: r1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if r2.DeltaSource != domain.DeltaSourceGit || !slices.Equal(r2.DeltaScope, []string{"adapters/x.go"}) {
		t.Fatalf("delta de git real: %+v", r2)
	}
	inherited, _ := reviews.InheritedFindings(project, r2.ID)
	if len(inherited) != 2 {
		t.Fatalf("se heredan los dos SUSPECT: %+v", inherited)
	}
	refPorArchivo := map[string]int64{}
	for _, s := range inherited {
		refPorArchivo[s.Files[0]] = s.ConsensusID
	}
	submit(r2, domain.ReviewerA, f("A-1", "domain/rules.go:1", refPorArchivo["domain/rules.go"]), f("A-2", "adapters/x.go:1", refPorArchivo["adapters/x.go"]))
	submit(r2, domain.ReviewerB)

	var reabierto int64
	for _, fd := range findingIDs(r2) {
		switch fd.Location {
		case "domain/rules.go:1":
			if !fd.Settled {
				t.Fatalf("archivo sin cambios: asentado: %+v", fd)
			}
		case "adapters/x.go:1":
			if fd.Settled {
				t.Fatalf("archivo cambiado: reabierto: %+v", fd)
			}
			reabierto = fd.ID
		}
	}
	// Solo el reabierto se clasifica: el asentado no se vuelve a adjudicar.
	if _, err := usecases.BuildConsensus(reviews, ledger, usecases.BuildConsensusInput{Project: project, ReviewID: r2.ID,
		Unmatched: []usecases.ConsensusUnmatched{{FindingID: reabierto, Status: domain.ConsensusSuspect, Severity: domain.SeverityHigh}}}); err != nil {
		t.Fatalf("clasificar solo el reabierto debe bastar: %v", err)
	}
	if done, err := usecases.FinalizeReview(reviews, ledger, project, r2.ID); err != nil || done.Verdict != domain.VerdictApproved {
		t.Fatalf("la revisión 2 aprueba: %+v %v", done, err)
	}
	if c, _ := ledger.ListAllConsensusFindings(project, r2.ID); len(c) != 1 {
		t.Fatalf("la revisión 2 adjudica un solo hallazgo, no dos: %+v", c)
	}

	// HCD-035: el binario real informa del ahorro de una revisión enlazada.
	cmd := exec.Command(buildMemBinary(t), "review", "--commit", commit2, "--since", r1.ID)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GOMEMORY_DATA_HOME="+data, "GOMEMORY_NO_CODE_REFRESH=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem review --commit --since: %v\n%s", err, out)
	}
	var avoided, scope int
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "ahorro estimado: ") {
			if _, err := fmt.Sscanf(line, "ahorro estimado: %d tokens (alcance %d", &avoided, &scope); err != nil {
				t.Fatalf("línea de ahorro ilegible %q: %v", line, err)
			}
		}
	}
	if scope == 0 || avoided <= 0 || avoided >= scope {
		t.Fatalf("ahorro medido: evitados %d de %d\n%s", avoided, scope, out)
	}
}
