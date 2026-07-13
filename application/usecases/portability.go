package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// ExportProject arma un bundle portable con TODAS las memorias y relaciones del
// proyecto. Los RefID del bundle son los ids reales de origen; sirven para
// remapear las relaciones al importar. Normaliza los filepaths a `/` (cross-OS).
func ExportProject(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, project string) (domain.ExportBundle, error) {
	mems, err := memRepo.ListAll(project)
	if err != nil {
		return domain.ExportBundle{}, err
	}
	rels, err := relRepo.ListAll(project)
	if err != nil {
		return domain.ExportBundle{}, err
	}

	bundle := domain.ExportBundle{
		Version:    domain.ExportVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Source:     project,
	}
	for _, m := range mems {
		bundle.Memories = append(bundle.Memories, domain.ExportMemory{
			RefID:        m.ID,
			Type:         string(m.Type),
			Title:        m.Title,
			Content:      m.Content,
			Filepath:     filepath.ToSlash(m.Filepath),
			OriginPrompt: m.OriginPrompt,
			CreatedAt:    m.CreatedAt,
			UpdatedAt:    m.UpdatedAt,
		})
	}
	for _, r := range rels {
		bundle.Relations = append(bundle.Relations, domain.ExportRelation{
			RefA:       r.MemoryIDA,
			RefB:       r.MemoryIDB,
			Relation:   string(r.Relation),
			Confidence: r.Confidence,
			Reasoning:  r.Reasoning,
			CreatedAt:  r.CreatedAt,
		})
	}
	return bundle, nil
}

// EncodeBundle serializa el bundle como JSON indentado (portable).
func EncodeBundle(w io.Writer, bundle domain.ExportBundle) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(bundle)
}

// DecodeBundle deserializa un bundle desde JSON.
func DecodeBundle(r io.Reader) (domain.ExportBundle, error) {
	var b domain.ExportBundle
	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return domain.ExportBundle{}, err
	}
	return b, nil
}

// memoryHash identifica una memoria por su contenido (no por id), para dedup al
// importar: dos memorias con el mismo tipo+título+contenido se consideran la misma.
func memoryHash(memType, title, content string) string {
	sum := sha256.Sum256([]byte(memType + "\x00" + title + "\x00" + content))
	return hex.EncodeToString(sum[:])
}

// ImportBundle importa un bundle en targetProject. Append con dedup por contenido:
// las memorias ya presentes se saltan (pero se mapean para las relaciones);
// preserva timestamps de origen, remapea el proyecto y NO forma sinapsis
// automáticas (las relaciones se importan explícitas, con ids remapeados).
func ImportBundle(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, targetProject string, bundle domain.ExportBundle) (domain.ImportReport, error) {
	var rep domain.ImportReport

	existing, err := memRepo.ListAll(targetProject)
	if err != nil {
		return rep, err
	}
	byHash := make(map[string]int64, len(existing))
	for _, m := range existing {
		byHash[memoryHash(string(m.Type), m.Title, m.Content)] = m.ID
	}

	// resolved: RefID de origen → id real en el destino (nuevo o ya existente).
	resolved := make(map[int64]int64, len(bundle.Memories))
	for _, em := range bundle.Memories {
		h := memoryHash(em.Type, em.Title, em.Content)
		if id, ok := byHash[h]; ok {
			resolved[em.RefID] = id
			rep.MemoriesSkipped++
			continue
		}
		id, err := memRepo.ImportMemory(&domain.Memory{
			Project:      targetProject,
			Type:         domain.MemoryType(em.Type),
			Title:        em.Title,
			Content:      em.Content,
			Filepath:     em.Filepath,
			OriginPrompt: em.OriginPrompt,
			CreatedAt:    em.CreatedAt,
			UpdatedAt:    em.UpdatedAt,
		})
		if err != nil {
			return rep, err
		}
		resolved[em.RefID] = id
		byHash[h] = id // evita duplicar dentro del mismo bundle
		rep.MemoriesImported++
	}

	for _, er := range bundle.Relations {
		a, okA := resolved[er.RefA]
		b, okB := resolved[er.RefB]
		if !okA || !okB {
			rep.RelationsSkipped++
			continue
		}
		if existingRel, _ := relRepo.GetByPair(targetProject, a, b); existingRel != nil {
			rep.RelationsSkipped++
			continue
		}
		if _, err := relRepo.ImportRelation(&domain.Relation{
			Project:    targetProject,
			MemoryIDA:  a,
			MemoryIDB:  b,
			Relation:   domain.ValidRelationType(er.Relation),
			Confidence: er.Confidence,
			Reasoning:  er.Reasoning,
			CreatedAt:  er.CreatedAt,
		}); err != nil {
			return rep, err
		}
		rep.RelationsImported++
	}

	return rep, nil
}
