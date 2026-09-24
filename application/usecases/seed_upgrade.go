package usecases

import (
	"errors"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// SeedUpgradeReport resume qué hizo UpgradePristineSeeds, por clave de tópico.
type SeedUpgradeReport struct {
	// Upgraded son las semillas intactas llevadas a la plantilla actual.
	Upgraded []string
	// Customized son las semillas que el equipo editó y que difieren de la
	// plantilla actual: no se tocan, pero hay una versión nueva disponible.
	Customized []string
}

// UpgradePristineSeeds lleva a la plantilla actual las semillas que nadie
// editó. Es lo que permite que una plantilla nueva (por ejemplo, la
// constitución) llegue sola con `mem update` a los proyectos ya sembrados.
//
// Complementa a SeedDefaults sin relajar su invariante: esta sigue sin pisar
// nunca un documento existente. Aquí solo se reemplaza una semilla cuyo
// contenido es, intacto, una plantilla que distribuyó una versión anterior
// (huella en Seed.PreviousDefaultSHA256). Ese texto no es del equipo, es del
// binario viejo, y actualizarlo no pierde trabajo de nadie. Cualquier
// diferencia, aunque sea mínima, marca la semilla como personalizada y la deja
// como está (el texto del equipo gana, research.md §R5 de la feature 021).
//
// Escribe por la misma vía inerte que `mem docs reset`: sin sinapsis ni
// publicación al ADR externo. Es oportunista, igual que la siembra: acumula
// errores sin abortar el resto.
func UpgradePristineSeeds(
	seeder ports.MemorySeeder,
	topics ports.MemoryTopicQuerier,
	project string,
	seeds []Seed,
) (SeedUpgradeReport, error) {
	var rep SeedUpgradeReport
	if seeder == nil || topics == nil {
		return rep, nil
	}

	var errs []error
	for _, s := range seeds {
		if strings.TrimSpace(s.Content) == "" {
			continue // plantilla no embebida en este binario
		}
		existente, err := topics.ByTopicKey(project, s.TopicKey)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if existente == nil {
			continue // la siembra la crea; no es una actualización
		}
		if strings.TrimSpace(existente.Content) == strings.TrimSpace(s.Content) {
			continue
		}
		doc := domain.PinnedDoc{PreviousDefaultSHA256: s.PreviousDefaultSHA256}
		if !doc.IsPreviousDefault(existente.Content) {
			rep.Customized = append(rep.Customized, s.TopicKey)
			continue
		}
		if _, err := ImportPinnedDoc(seeder, topics, project, s.TopicKey, s.Type, s.Title, s.Content); err != nil {
			errs = append(errs, err)
			continue
		}
		rep.Upgraded = append(rep.Upgraded, s.TopicKey)
	}
	return rep, errors.Join(errs...)
}
