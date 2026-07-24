package domain

import (
	"strings"
	"testing"
)

const sampleDoc = `## PURPOSE

Esto es texto libre escrito a mano, sin bloques.

## STACK

## ARCHITECTURE

<!-- gomemory:id=42 -->
### Usar SQLite WAL
decisión de concurrencia de escritura

### Convención de logging
escrita directo por una persona, sin marcador

## PATTERNS

## TRADEOFFS

<!-- gomemory:id=57 -->
### Preferir simplicidad sobre flexibilidad
menos superficie de configuración

## PHILOSOPHY
`

func TestParseADRDocument_AllSixSectionsPresent(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	if len(doc.Sections) != 6 {
		t.Fatalf("esperaba 6 secciones, hubo %d", len(doc.Sections))
	}
	names := make([]string, len(doc.Sections))
	for i, s := range doc.Sections {
		names[i] = s.Name
	}
	want := []string{"PURPOSE", "STACK", "ARCHITECTURE", "PATTERNS", "TRADEOFFS", "PHILOSOPHY"}
	for i, w := range want {
		if names[i] != w {
			t.Fatalf("sección %d = %q, se esperaba %q (orden: %v)", i, names[i], w, names)
		}
	}
}

func TestParseADRDocument_MarkedBlockHasMemoryID(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	arch := doc.Section("ARCHITECTURE")
	if len(arch.Blocks) != 2 {
		t.Fatalf("ARCHITECTURE debería tener 2 bloques, hubo %d: %+v", len(arch.Blocks), arch.Blocks)
	}
	first := arch.Blocks[0]
	if first.MemoryID == nil || *first.MemoryID != 42 {
		t.Fatalf("primer bloque de ARCHITECTURE debería tener MemoryID=42, hubo %+v", first)
	}
	if first.Heading != "Usar SQLite WAL" {
		t.Fatalf("heading inesperado: %q", first.Heading)
	}
	if !strings.Contains(first.Body, "concurrencia de escritura") {
		t.Fatalf("body inesperado: %q", first.Body)
	}
}

func TestParseADRDocument_UnmarkedBlockHasNilMemoryID(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	arch := doc.Section("ARCHITECTURE")
	second := arch.Blocks[1]
	if second.MemoryID != nil {
		t.Fatalf("bloque sin marcador no debería tener MemoryID, hubo %v", *second.MemoryID)
	}
	if second.Heading != "Convención de logging" {
		t.Fatalf("heading inesperado: %q", second.Heading)
	}
}

func TestParseADRDocument_FreeTextWithoutHeadingsBecomesAnonymousBlock(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	purpose := doc.Section("PURPOSE")
	if len(purpose.Blocks) != 1 {
		t.Fatalf("PURPOSE (texto libre sin ###) debería quedar como 1 bloque anónimo, hubo %d", len(purpose.Blocks))
	}
	if purpose.Blocks[0].Heading != "" {
		t.Fatalf("bloque de texto libre no debería tener heading, tuvo %q", purpose.Blocks[0].Heading)
	}
	if !strings.Contains(purpose.Blocks[0].Body, "texto libre escrito a mano") {
		t.Fatalf("se perdió el contenido libre: %q", purpose.Blocks[0].Body)
	}
}

func TestParseADRDocument_EmptySection_NoBlocks(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	if blocks := doc.Section("STACK").Blocks; len(blocks) != 0 {
		t.Fatalf("STACK está vacía en la fixture, no debería tener bloques: %+v", blocks)
	}
}

func TestParseADRDocument_Empty(t *testing.T) {
	doc := ParseADRDocument("")
	if len(doc.Sections) != 6 {
		t.Fatalf("un documento vacío igual debería traer las 6 secciones fijas (vacías), hubo %d", len(doc.Sections))
	}
	for _, s := range doc.Sections {
		if len(s.Blocks) != 0 {
			t.Fatalf("sección %q de un documento vacío no debería tener bloques", s.Name)
		}
	}
}

func TestADRDocument_RenderRoundTrip_PreservesMarkedAndAnonymousBlocks(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	rendered := doc.Render()
	reparsed := ParseADRDocument(rendered)

	arch := reparsed.Section("ARCHITECTURE")
	if len(arch.Blocks) != 2 || arch.Blocks[0].MemoryID == nil || *arch.Blocks[0].MemoryID != 42 {
		t.Fatalf("el round-trip perdió el bloque marcado de ARCHITECTURE: %+v", arch.Blocks)
	}
	tradeoffs := reparsed.Section("TRADEOFFS")
	if len(tradeoffs.Blocks) != 1 || tradeoffs.Blocks[0].MemoryID == nil || *tradeoffs.Blocks[0].MemoryID != 57 {
		t.Fatalf("el round-trip perdió el bloque marcado de TRADEOFFS: %+v", tradeoffs.Blocks)
	}
	purpose := reparsed.Section("PURPOSE")
	if len(purpose.Blocks) != 1 || !strings.Contains(purpose.Blocks[0].Body, "texto libre escrito a mano") {
		t.Fatalf("el round-trip perdió el texto libre de PURPOSE: %+v", purpose.Blocks)
	}
}

func TestADRDocument_UpsertBlock_AppendsWhenNew(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	doc.UpsertBlock("ARCHITECTURE", 99, "Nueva decisión", "contenido nuevo")

	arch := doc.Section("ARCHITECTURE")
	if len(arch.Blocks) != 3 {
		t.Fatalf("esperaba 3 bloques en ARCHITECTURE tras el upsert, hubo %d", len(arch.Blocks))
	}
	last := arch.Blocks[len(arch.Blocks)-1]
	if last.MemoryID == nil || *last.MemoryID != 99 || last.Heading != "Nueva decisión" {
		t.Fatalf("el bloque nuevo no quedó como se esperaba: %+v", last)
	}
}

func TestADRDocument_UpsertBlock_UpdatesExistingByMemoryID(t *testing.T) {
	doc := ParseADRDocument(sampleDoc)
	doc.UpsertBlock("ARCHITECTURE", 42, "Usar SQLite WAL (revisado)", "contenido actualizado")

	arch := doc.Section("ARCHITECTURE")
	if len(arch.Blocks) != 2 {
		t.Fatalf("actualizar un bloque existente no debería crear uno nuevo, hubo %d bloques", len(arch.Blocks))
	}
	if arch.Blocks[0].Heading != "Usar SQLite WAL (revisado)" || !strings.Contains(arch.Blocks[0].Body, "contenido actualizado") {
		t.Fatalf("el bloque no se actualizó: %+v", arch.Blocks[0])
	}
}

func TestADRDocument_Render_AlwaysEmitsAllSixSections(t *testing.T) {
	doc := ParseADRDocument("")
	rendered := doc.Render()
	for _, name := range []string{"PURPOSE", "STACK", "ARCHITECTURE", "PATTERNS", "TRADEOFFS", "PHILOSOPHY"} {
		if !strings.Contains(rendered, "## "+name) {
			t.Fatalf("Render() de un documento vacío debería incluir '## %s'", name)
		}
	}
}
