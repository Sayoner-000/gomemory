package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	"mem/application/usecases"
	"mem/domain"
)

func CmdCompare(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("uso: mem compare [flags] <id1> <id2>\n   o: mem compare list [-n N]\nFlags: -r relation, -c confidence, -m reasoning")
	}

	if args[0] == "list" {
		cmdCompareList(deps, args[1:])
		return
	}

	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	relation := fs.String("r", "related", "Relación: related|compatible|scoped|conflicts_with|supersedes|not_conflict")
	confidence := fs.Float64("c", 1.0, "Confianza (0.0-1.0)")
	reasoning := fs.String("m", "", "Razonamiento del veredicto")
	fs.Parse(args)

	positional := fs.Args()
	if len(positional) < 2 {
		fail("se requieren dos IDs de memoria\nEjemplo: mem compare -r supersedes -c 0.9 -m \"razón\" 1 2")
	}

	id1, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		fail("ID inválido: %s", positional[0])
	}
	id2, err := strconv.ParseInt(positional[1], 10, 64)
	if err != nil {
		fail("ID inválido: %s", positional[1])
	}

	if id1 == id2 {
		fail("no se puede comparar una memoria consigo misma")
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)

	// Verify both memories exist
	mems, err := deps.MemoryRepo.List(project, 200)
	if err != nil {
		fail("listar memorias: %v", err)
	}
	foundA := false
	foundB := false
	for _, m := range mems {
		if m.ID == id1 {
			foundA = true
		}
		if m.ID == id2 {
			foundB = true
		}
	}
	if !foundA {
		fail("memoria %d no encontrada en el proyecto '%s'", id1, project)
	}
	if !foundB {
		fail("memoria %d no encontrada en el proyecto '%s'", id2, project)
	}

	relType := domain.ValidRelationType(*relation)
	reasonText := *reasoning
	if reasonText == "" {
		reasonText = fmt.Sprintf("Veredicto: %s (confianza: %.2f)", relType, *confidence)
	}

	rel, updated, err := usecases.RecordVerdict(deps.RelationRepo, project, id1, id2, relType, *confidence, reasonText)
	if err != nil {
		fail("%v", err)
	}
	if updated {
		fmt.Printf("✓ Relación actualizada (id=%d): %s ↔ %s → %s\n", rel.ID, positional[0], positional[1], relType)
	} else {
		fmt.Printf("✓ Relación guardada (id=%d): %s ↔ %s → %s\n", rel.ID, positional[0], positional[1], relType)
	}
	fmt.Printf("  Razonamiento: %s\n", reasonText)
}

func cmdCompareList(deps *Deps, args []string) {
	fs := flag.NewFlagSet("compare list", flag.ContinueOnError)
	limit := fs.Int("n", 20, "Número de relaciones")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)
	rels, err := deps.RelationRepo.List(project, *limit)
	if err != nil {
		fail("listar relaciones: %v", err)
	}

	if len(rels) == 0 {
		fmt.Println("Sin relaciones guardadas. Crea una con: mem compare -r related -m \"razón\" <id1> <id2>")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tMemoria A\tMemoria B\tRelación\tConfianza\tRazonamiento\n")
	fmt.Fprintf(w, "--\t---------\t---------\t--------\t---------\t------------\n")
	for _, r := range rels {
		reason := r.Reasoning
		if len(reason) > 40 {
			reason = reason[:37] + "..."
		}
		date := r.CreatedAt
		if len(date) > 10 {
			date = date[:10]
		}
		fmt.Fprintf(w, "%d\t%d\t%d\t%s\t%.2f\t%s\n", r.ID, r.MemoryIDA, r.MemoryIDB, string(r.Relation), r.Confidence, reason)
	}
	w.Flush()
	fmt.Printf("\n(%d relaciones)\n", len(rels))
}

var relationDescriptions = map[string]string{
	"related":       "Las memorias están semanticamente relacionadas",
	"compatible":    "Las memorias son compatibles entre sí",
	"scoped":        "Una memoria es un caso específico o alcance de la otra",
	"conflicts_with": "Las memorias entran en conflicto",
	"supersedes":    "Una memoria reemplaza o invalida a la otra",
	"not_conflict":  "Se evaluaron y no hay conflicto",
}
