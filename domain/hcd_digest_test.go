package domain

import (
	"strings"
	"testing"
	"time"
)

const digestSampleDoc = `# Filepath y topic_key dentro del contrato

ID: HCD-044
Tipo: fix
Carril: full

## Intención
Que ` + "`filepath`" + ` y ` + "`topic_key`" + ` nunca guarden texto privado.

## Reproducción
` + "`adapters/secondary/persistence/memory.go:111`" + ` escribe sin proteger.

## Alcance
Incluido: escrituras, lectura y saneamiento.
Excluido: otros campos.

## ¿Qué pasa si no lo hacemos?
El texto privado queda en la base.

## Alternativa más barata
Proteger solo la escritura nueva; se descarta porque deja las filas viejas.

## Historias
### US1. Metadatos sin texto privado
Como persona, quiero que no se guarde, para cumplir la promesa.
- C-01: Dado X, cuando Y, entonces Z.

## Aclaraciones
- [x] Q1. Escritura: ¿rechazar o redactar? → Rechazar (Recommended)
- [ ] Q2. ¿Algo abierto?

## Slices
### VS-01. Escritura
Tareas: 1
Criterios: C-01

## Checklist
- [x] 1. Rechazar → ` + "`go test ./domain/`" + `

## Evidencia
- ` + "`adapters/ignorado/en_evidencia.go`" + `

## Siguiente
pendiente
`

func TestRenderHCDDigest(t *testing.T) {
	doc, _ := ParseUnitDoc([]byte(digestSampleDoc), KindFix, LaneFull)
	created := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	u := Unit{ID: "HCD-044", Title: "Filepath y topic_key dentro del contrato", Kind: KindFix, Lane: LaneFull, Why: "Privacidad, riesgo alto", State: UnitClosed, DocPath: "hcd/HCD-044-x.md", CreatedAt: created}
	approvals := []Approval{
		{Action: ApprovalApprove, Revision: "58ec5bef1b3e", Quote: "Apruebo 58ec5bef1b3e", CreatedAt: created},
		{Action: ApprovalClose, Revision: "2db1263e809f", Quote: "Cierro 2db1263e809f", CreatedAt: created.Add(time.Hour)},
	}

	d := RenderHCDDigest(u, doc, approvals)

	if d.Title != "HCD-044 · Filepath y topic_key dentro del contrato (cerrada)" {
		t.Fatalf("título = %q", d.Title)
	}
	for _, want := range []string{
		"Tipo: fix · Carril: full · Estado: cerrada · Inicio: 2026-11-06",
		"Intención: Que `filepath` y `topic_key` nunca guarden texto privado.",
		"Motivo del carril: Privacidad, riesgo alto",
		"Alcance: Incluido: escrituras, lectura y saneamiento. Excluido: otros campos.",
		"Alternativa descartada: Proteger solo la escritura nueva",
		"- Q1. Escritura: ¿rechazar o redactar? → Rechazar (Recommended)",
		"- Q2. ¿Algo abierto? → (abierta)",
		"definición aprobada 58ec5bef1b3e",
		"cierre 2db1263e809f",
		"Archivos: adapters/secondary/persistence/memory.go",
	} {
		if !strings.Contains(d.Content, want) {
			t.Errorf("la ficha no contiene %q:\n%s", want, d.Content)
		}
	}
	if strings.Contains(d.Content, "Apruebo") || strings.Contains(d.Content, "en_evidencia") {
		t.Errorf("la ficha incluye citas o rutas de la evidencia:\n%s", d.Content)
	}
	if strings.Join(d.Files, ",") != "adapters/secondary/persistence/memory.go" {
		t.Errorf("archivos = %v", d.Files)
	}
	if HCDDigestTopic(u.ID) != "hcd/HCD-044" {
		t.Errorf("topic = %q", HCDDigestTopic(u.ID))
	}

	long := u
	long.Why = strings.Repeat("motivo largo ", 1000)
	if got := RenderHCDDigest(long, doc, nil); len([]rune(got.Content)) > HCDDigestMaxContent {
		t.Errorf("la ficha supera el tope: %d runas", len([]rune(got.Content)))
	}
}

func TestIsHCDStepMemoryTitle(t *testing.T) {
	for title, want := range map[string]bool{
		"HCD-041: carril fast":                         true,
		"HCD-036: carril full":                         true,
		"HCD-041: aclaración Q1":                       true,
		"HCD-044: aclaración Q12":                      true,
		"HCD-041: definición aprobada":                 true,
		"HCD-041: cerrada":                             true,
		"HCD-001: cancelada":                           true,
		"HCD-020: promovida a la ceremonia completa":   true,
		"HCD-041: el grafo propio guarda las llamadas": false,
		"HCD-041: cerrada y verificada":                false,
		"HCD-041 · Grafo propio (cerrada)":             false,
		"Nota sobre HCD-041: cerrada":                  false,
		"HCD-041: carril lento":                        false,
	} {
		if got := IsHCDStepMemoryTitle(title); got != want {
			t.Errorf("IsHCDStepMemoryTitle(%q) = %v, quiero %v", title, got, want)
		}
	}
}
