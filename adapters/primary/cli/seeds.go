package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// seedsFromCatalog construye las semillas recorriendo domain.PinnedDocs y
// leyendo el contenido por defecto de las plantillas embebidas. Una plantilla
// ausente (TemplatesFS nil en algunos tests) produce contenido vacío, que
// SeedDefaults omite en silencio.
func seedsFromCatalog() []usecases.Seed {
	out := make([]usecases.Seed, 0, len(domain.PinnedDocs))
	for _, d := range domain.PinnedDocs {
		out = append(out, usecases.Seed{
			TopicKey: d.TopicKey,
			Type:     d.Type,
			Title:    d.Title,
			Content:  embeddedTemplate(d.Template),

			PreviousDefaultSHA256: d.PreviousDefaultSHA256,
		})
	}
	return out
}

// seedDeps extrae del repositorio las dos capacidades que la siembra necesita.
// Aserción de tipo en vez de ensanchar ports.MemoryRepository: mismo patrón que
// el composition root usa con ADRSyncProvider.
func seedDeps(deps *Deps) (ports.MemorySeeder, ports.MemoryTopicQuerier, bool) {
	seeder, ok1 := deps.MemoryRepo.(ports.MemorySeeder)
	topics, ok2 := deps.MemoryRepo.(ports.MemoryTopicQuerier)
	return seeder, topics, ok1 && ok2
}

// seedReport resume una siembra: las semillas creadas, las intactas que se
// llevaron a la plantilla actual y las personalizadas que tienen una versión
// nueva disponible.
type seedReport struct {
	Created    []string
	Upgraded   []string
	Customized []string
}

// seedProject siembra las memorias por defecto que falten y actualiza las
// intactas de versiones anteriores (usecases.UpgradePristineSeeds), de modo
// que una plantilla nueva llega sola con `mem update`.
//
// Es una capa OPORTUNISTA: quien la llama informa el resultado pero nunca
// aborta por él. Se invoca desde dos sitios —`mem install` y el arranque del
// servidor MCP— porque desde v1.9 mucha gente registra el MCP en ámbito global
// y no ejecuta `install` nunca (research.md §R6).
func seedProject(deps *Deps, project string) (seedReport, error) {
	seeder, topics, ok := seedDeps(deps)
	if !ok {
		return seedReport{}, nil
	}
	seeds := seedsFromCatalog()
	created, errSeed := usecases.SeedDefaults(seeder, topics, project, seeds)
	upg, errUpg := usecases.UpgradePristineSeeds(seeder, topics, project, seeds)
	return seedReport{Created: created, Upgraded: upg.Upgraded, Customized: upg.Customized}, errors.Join(errSeed, errUpg)
}

// aliasForTopic traduce una clave de tópico al alias que teclea la persona.
func aliasForTopic(topicKey string) string {
	if d, ok := domain.PinnedDocByTopicKey(topicKey); ok {
		return d.Alias
	}
	return topicKey
}

// CmdSeed siembra las memorias por defecto del proyecto del directorio actual.
//
// Existe como subcomando propio por una razón concreta: `install` está en
// rootIndependentCommands, así que se despacha SIN contenedor y su Deps no trae
// MemoryRepo — no puede escribir en la memoria del proyecto destino, que además
// suele ser distinto del directorio actual. El instalador lo invoca como
// subproceso con cwd en el destino, exactamente igual que ya hace con `init`.
//
// También sirve de escape manual: `mem seed` recrea una semilla borrada.
func CmdSeed(deps *Deps, _ []string) {
	rep, err := seedProject(deps, deps.Project)
	if err != nil {
		fail("sembrar memorias por defecto: %v", err)
	}
	printSeedReport(os.Stdout, rep)
	if doc, ok := domain.PinnedDocByAlias("constitution"); ok {
		refreshSpeckitConstitution(os.Stdout, deps.Root, doc, embeddedTemplate(doc.Template))
	}
}

// refreshSpeckitConstitution mantiene al día la copia que dejó
// `mem constitution --sync` en .specify/memory/constitution.md, que es la que
// leen los comandos de spec-kit. Mismo criterio que UpgradePristineSeeds: se
// reescribe solo si es, intacta, una plantilla anterior; si el equipo la
// editó, se avisa y no se toca. Nunca crea .specify/.
func refreshSpeckitConstitution(w io.Writer, root string, doc domain.PinnedDoc, current string) {
	if root == "" || strings.TrimSpace(current) == "" {
		return
	}
	ruta := filepath.Join(root, ".specify", "memory", "constitution.md")
	data, err := os.ReadFile(ruta)
	if err != nil {
		return
	}
	previo := string(data)
	if strings.TrimSpace(previo) == strings.TrimSpace(current) {
		return
	}
	if !doc.IsPreviousDefault(previo) {
		_, _ = fmt.Fprintf(w, "ℹ️  .specify/memory/constitution.md tiene ediciones del equipo y no se modificó; "+
			"para reflejar la constitución vigente: mem constitution --sync\n")
		return
	}
	if err := os.WriteFile(ruta, []byte(current), 0o644); err != nil {
		_, _ = fmt.Fprintf(w, "⚠️  no se pudo actualizar %s: %v\n", ruta, err)
		return
	}
	_, _ = fmt.Fprintln(w, "✓ .specify/memory/constitution.md actualizada a la versión por defecto de este binario")
}

// printSeedReport informa la siembra. Es lo que ve la persona durante
// `mem install` y `mem update`, que ejecutan `mem seed` como subproceso.
func printSeedReport(w io.Writer, rep seedReport) {
	if len(rep.Created) > 0 {
		_, _ = fmt.Fprintf(w, "✓ %d memoria(s) por defecto sembradas\n", len(rep.Created))
	}
	for _, k := range rep.Upgraded {
		_, _ = fmt.Fprintf(w, "✓ %s actualizada a la versión por defecto de este binario (no tenía ediciones propias)\n", aliasForTopic(k))
	}
	for _, k := range rep.Customized {
		a := aliasForTopic(k)
		_, _ = fmt.Fprintf(w, "ℹ️  %s tiene ediciones del equipo y no se modificó. Hay una versión por defecto nueva; "+
			"para adoptarla: mem docs export %s -o respaldo.md && mem docs reset %s\n", a, a, a)
	}
	if len(rep.Created)+len(rep.Upgraded)+len(rep.Customized) == 0 {
		_, _ = fmt.Fprintln(w, "Semillas ya presentes y al día.")
	}
}
