package usecases

import (
	"context"
	"crypto/sha256"
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// ImportADRs trae al proyecto de gomemory los bloques del documento único de
// ADR del proveedor externo que NO se originaron en gomemory (feature 010,
// Historia 2, sentido proveedor→gomemory). Coordina tres puertos
// (ADRSyncProvider, ADRSyncRepository, MemoryRepository) — a diferencia del
// export (que vive en el choke point InsertMemory, un paso más sobre lo que
// ya hace), esta orquestación sí amerita su propia capa de caso de uso.
//
// Idempotente: un bloque ya importado sin cambios no genera trabajo; un
// bloque marcado como gomemory-origin nunca se reimporta (evita el bucle de
// sincronización). Si el documento cambió del lado del proveedor y la copia
// local en gomemory NO fue editada desde el último sync, se actualiza en
// el lugar; si AMBOS lados cambiaron, se conserva la copia local y se deja
// constancia de un conflicto resuelto (ninguna versión se descarta en
// silencio).
//
// Devuelve error solo si no se pudo leer el documento (proveedor no
// disponible) — el llamador (hook de refresco) decide degradar en silencio,
// igual que el resto de esta feature.
func ImportADRs(ctx context.Context, provider ports.ADRSyncProvider, adrRepo ports.ADRSyncRepository, memRepo ports.MemoryRepository, project string) error {
	docContent, err := provider.GetDocument(ctx)
	if err != nil {
		return fmt.Errorf("import adrs: %w", err)
	}

	doc := domain.ParseADRDocument(docContent)
	for _, section := range doc.Sections {
		for _, block := range section.Blocks {
			if block.MemoryID != nil {
				continue // origen gomemory: nunca se reimporta (evita el bucle)
			}
			importBlock(provider, adrRepo, memRepo, project, section.Name, block)
		}
	}
	return nil
}

func importBlock(provider ports.ADRSyncProvider, adrRepo ports.ADRSyncRepository, memRepo ports.MemoryRepository, project, section string, block domain.ADRBlock) {
	if block.Heading == "" && block.Body == "" {
		return
	}
	blockKey := providerBlockKey(section, block.Heading)

	existing, _ := adrRepo.GetByBlockKey(project, provider.Name(), blockKey)
	if existing == nil {
		importNewBlock(provider, adrRepo, memRepo, project, section, blockKey, block)
		return
	}
	updateImportedBlock(adrRepo, memRepo, project, existing, block)
}

func importNewBlock(provider ports.ADRSyncProvider, adrRepo ports.ADRSyncRepository, memRepo ports.MemoryRepository, project, section, blockKey string, block domain.ADRBlock) {
	id, err := memRepo.ImportMemory(&domain.Memory{
		Project: project, Type: domain.Architecture, Title: block.Heading, Content: block.Body,
	})
	if err != nil {
		return // best-effort: se reintenta en el próximo ciclo de refresco
	}
	// Hashear lo REALMENTE persistido (post-redacción), no block.Body crudo,
	// para que la próxima comparación no dispare un falso conflicto.
	stored, _ := memRepo.Get(project, id)
	hash := contentHashOf(stored)

	memID := id
	adrRepo.Insert(&domain.ADRSyncRecord{
		Project: project, MemoryID: &memID, Provider: provider.Name(),
		Section: section, BlockKey: blockKey, Origin: domain.SyncOriginProvider,
		Status: domain.SyncStatusOK, ContentHash: hash,
	})
}

func updateImportedBlock(adrRepo ports.ADRSyncRepository, memRepo ports.MemoryRepository, project string, existing *domain.ADRSyncRecord, block domain.ADRBlock) {
	newHash := contentHash(block.Body)
	if newHash == existing.ContentHash {
		return // sin cambios del lado del proveedor
	}
	if existing.MemoryID == nil {
		return // no debería pasar; sin memoria que actualizar
	}

	local, _ := memRepo.Get(project, *existing.MemoryID)
	if local != nil && contentHash(local.Content) != existing.ContentHash {
		// La copia local cambió desde el último sync (edición humana) Y el
		// proveedor también cambió: conflicto. Se conserva la copia local sin
		// pisarla — ninguna de las dos versiones se pierde: la del proveedor
		// sigue en su documento, disponible para revisión manual.
		adrRepo.UpdateStatus(existing.ID, domain.SyncStatusConflictResolved, existing.ContentHash)
		return
	}

	if err := memRepo.UpdateContent(project, *existing.MemoryID, block.Heading, block.Body); err != nil {
		adrRepo.UpdateStatus(existing.ID, domain.SyncStatusFailed, existing.ContentHash)
		return
	}
	stored, _ := memRepo.Get(project, *existing.MemoryID)
	adrRepo.UpdateStatus(existing.ID, domain.SyncStatusOK, contentHashOf(stored))
}

func contentHashOf(m *domain.Memory) string {
	if m == nil {
		return ""
	}
	return contentHash(m.Content)
}

func contentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// providerBlockKey es la identidad estable de un bloque sin marcador
// gomemory (origen proveedor): no hay ID real que usar, así que se deriva de
// sección+heading. Si alguien renombra el heading a mano, se trata como un
// bloque nuevo — degradación aceptable, documentada en research.md §3.
func providerBlockKey(section, heading string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(section+"|"+heading)))
}
