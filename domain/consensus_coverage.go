package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ConsensusSource es un hallazgo de la ronda activa, reducido a lo que la validación
// de cobertura necesita saber de él. El dominio no lee del ledger: recibe las fuentes
// ya materializadas y decide.
type ConsensusSource struct {
	FindingID int64
	Reviewer  Reviewer
	Severity  Severity
	Claim     string
	// Confirmable resume Finding.Confirmable(): si el hallazgo trae severidad,
	// clase de evidencia y al menos una evidencia no vacía.
	Confirmable bool
}

// ConsensusPair es una clasificación que cubre dos hallazgos, uno de cada revisor.
type ConsensusPair struct {
	Status     ConsensusStatus
	FindingIDA int64
	FindingIDB int64
	Claim      string
	// DeclaredSeverity es informativa. Si viene y no coincide con la derivada de
	// las fuentes, la clasificación se rechaza en vez de degradarse en silencio.
	DeclaredSeverity Severity
}

// ConsensusSingle es una clasificación que cubre un solo hallazgo.
type ConsensusSingle struct {
	Status           ConsensusStatus
	FindingID        int64
	DeclaredSeverity Severity
}

// ConsensusClassification es la clasificación COMPLETA de una ronda. Ese "completa"
// es la diferencia con la versión anterior del protocolo, donde la entrada describía
// una parte cualquiera y nadie comprobaba qué quedaba fuera.
type ConsensusClassification struct {
	Matches   []ConsensusPair
	Unmatched []ConsensusSingle
}

// ValidateCoverage comprueba que la clasificación cubre exactamente una vez cada
// hallazgo de la ronda y devuelve los hallazgos de consenso ya derivados.
//
// Las cuatro reglas que aplica —cobertura total, unicidad, pertenencia a la ronda y
// severidad derivada— viven aquí y no en el caso de uso a propósito. Cuando estaban
// repartidas por BuildConsensus, cada una se comprobaba mientras se recorría la
// entrada, así que ninguna podía ver el conjunto: omitir un hallazgo no era un error,
// era simplemente no mencionarlo (FR-001 a FR-004).
//
// Los identificadores C-NNN se asignan tras ordenar por el menor ID de hallazgo
// fuente, no por orden de llegada. Reenviar la misma clasificación en otro orden
// producía antes identificadores distintos, y con ellos rompía las referencias que
// las correcciones y los re-juicios ya habían guardado.
func ValidateCoverage(
	sources []ConsensusSource, classification ConsensusClassification,
) ([]ConsensusFinding, error) {
	porID := make(map[int64]ConsensusSource, len(sources))
	for _, source := range sources {
		porID[source.FindingID] = source
	}

	asignados := make(map[int64]bool, len(sources))
	reclamar := func(id int64) error {
		if _, existe := porID[id]; !existe {
			return fmt.Errorf("el hallazgo %d no pertenece a la ronda activa", id)
		}
		if asignados[id] {
			return fmt.Errorf("el hallazgo %d se clasifica más de una vez", id)
		}
		asignados[id] = true
		return nil
	}

	var out []ConsensusFinding
	for _, match := range classification.Matches {
		if match.Status != ConsensusConfirmed && match.Status != ConsensusContradiction {
			return nil, fmt.Errorf("una clasificación emparejada debe ser CONFIRMED o CONTRADICTION")
		}
		if match.FindingIDA == match.FindingIDB {
			return nil, fmt.Errorf("el hallazgo %d no puede emparejarse consigo mismo", match.FindingIDA)
		}
		if err := reclamar(match.FindingIDA); err != nil {
			return nil, err
		}
		if err := reclamar(match.FindingIDB); err != nil {
			return nil, err
		}
		a, b := porID[match.FindingIDA], porID[match.FindingIDB]
		if a.Reviewer == b.Reviewer {
			return nil, fmt.Errorf("consensus sources must come from independent reviewers")
		}
		if match.Status == ConsensusConfirmed && (!a.Confirmable || !b.Confirmable) {
			return nil, fmt.Errorf("confirmed finding requires concrete evidence from both reviewers")
		}
		severidad := MaxSeverity(a.Severity, b.Severity)
		if err := comprobarSeveridadDeclarada(match.DeclaredSeverity, severidad); err != nil {
			return nil, err
		}
		claim := strings.TrimSpace(match.Claim)
		if claim == "" {
			claim = a.Claim
		}
		out = append(out, ConsensusFinding{
			Status: match.Status, Severity: severidad, Claim: claim,
			SourceFindingIDs: []int64{match.FindingIDA, match.FindingIDB},
		})
	}

	for _, single := range classification.Unmatched {
		if single.Status != ConsensusSuspect && single.Status != ConsensusInfo {
			return nil, fmt.Errorf("una clasificación no emparejada debe ser SUSPECT o INFO")
		}
		if err := reclamar(single.FindingID); err != nil {
			return nil, err
		}
		source := porID[single.FindingID]
		if err := comprobarSeveridadDeclarada(single.DeclaredSeverity, source.Severity); err != nil {
			return nil, err
		}
		out = append(out, ConsensusFinding{
			Status: single.Status, Severity: source.Severity, Claim: source.Claim,
			SourceFindingIDs: []int64{single.FindingID},
		})
	}

	if faltan := sinClasificar(sources, asignados); len(faltan) > 0 {
		return nil, fmt.Errorf("quedan %d hallazgos sin clasificar: %s", len(faltan), strings.Join(faltan, ", "))
	}

	ordenarPorFuente(out)
	for i := range out {
		out[i].ConsensusLocalID = fmt.Sprintf("C-%03d", i+1)
	}
	return out, nil
}

// comprobarSeveridadDeclarada rechaza cualquier severidad que no sea la derivada, no
// solo las menores. Aceptar una mayor «por prudencia» dejaría al orquestador
// inventando gravedad sin respaldo en ninguna fuente, que es el mismo problema al
// revés: el ledger afirmaría algo que sus hallazgos no sostienen.
func comprobarSeveridadDeclarada(declarada, derivada Severity) error {
	if strings.TrimSpace(string(declarada)) == "" {
		return nil
	}
	if declarada != derivada {
		return fmt.Errorf(
			"la severidad declarada %s no coincide con la derivada de las fuentes %s",
			declarada, derivada,
		)
	}
	return nil
}

func sinClasificar(sources []ConsensusSource, asignados map[int64]bool) []string {
	var faltan []string
	for _, source := range sources {
		if !asignados[source.FindingID] {
			faltan = append(faltan, strconv.FormatInt(source.FindingID, 10))
		}
	}
	sort.Strings(faltan)
	return faltan
}

// ordenarPorFuente da un orden total y determinista: por el menor ID de hallazgo que
// respalda cada clasificación.
func ordenarPorFuente(findings []ConsensusFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		return menorFuente(findings[i]) < menorFuente(findings[j])
	})
}

func menorFuente(finding ConsensusFinding) int64 {
	menor := int64(0)
	for i, id := range finding.SourceFindingIDs {
		if i == 0 || id < menor {
			menor = id
		}
	}
	return menor
}

// ClassificationFingerprint identifica una clasificación por su contenido, no por el
// orden en que llegó. Es lo que permite distinguir el reenvío exacto de una ronda
// —idempotente— de un intento de reemplazarla por otra distinta (FR-005).
func ClassificationFingerprint(findings []ConsensusFinding) string {
	entradas := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids := append([]int64(nil), finding.SourceFindingIDs...)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		partes := make([]string, 0, len(ids))
		for _, id := range ids {
			partes = append(partes, strconv.FormatInt(id, 10))
		}
		entradas = append(entradas, fmt.Sprintf("%s|%s|%s",
			finding.Status, finding.Severity, strings.Join(partes, ",")))
	}
	sort.Strings(entradas)
	sum := sha256.Sum256([]byte(strings.Join(entradas, "\n")))
	return hex.EncodeToString(sum[:])
}
