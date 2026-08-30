package domain

import (
	"fmt"
	"sort"
	"strings"
)

// ReviewLearning es el conocimiento reutilizable que sobrevive a una revisión.
//
// Su diseño es la garantía de FR-031 y AC-008: no hay campo donde quepa un
// transcript, un prompt en bruto ni una cadena de razonamiento. La exclusión no
// depende de que quien promueve se acuerde de filtrar — depende de que no exista
// el hueco. Lo que se conserva es lo que sirve dentro de seis meses: qué falló,
// por qué, cómo se arregló y cómo se comprobó.
type ReviewLearning struct {
	// ReviewID da trazabilidad al origen. Va en el CONTENIDO, no en la clave
	// de tópico: ver TopicKey.
	ReviewID     string
	Category     string
	Component    string
	Problem      string
	RootCause    string
	Resolution   string
	Verification []string
	Confidence   string
}

// reviewLearningTopicPrefix identifica estas memorias dentro del espacio de
// claves compartido con el resto de tópicos del proyecto.
const reviewLearningTopicPrefix = "review-learning"

// TopicKey agrupa por PATRÓN DE FALLO, no por revisión.
//
// Es la decisión que hace funcionar la deduplicación (FR-034): dos revisiones
// distintas que tropiezan con el mismo patrón comparten clave, así que la
// segunda actualiza la memoria de la primera en vez de añadir una casi idéntica.
// Incluir el review_id aquí daría una memoria por revisión y el conocimiento se
// fragmentaría justo cuando más veces se ha confirmado.
func (l ReviewLearning) TopicKey() string {
	componente := strings.TrimSpace(l.Component)
	if componente == "" {
		componente = "sin-componente"
	}
	return fmt.Sprintf("%s:%s:%s",
		reviewLearningTopicPrefix,
		strings.ToLower(strings.TrimSpace(l.Category)),
		strings.ToLower(componente),
	)
}

// Memory materializa el aprendizaje como memoria del proyecto.
//
// No redacta secretos aquí: InsertMemory ya aplica RedactSecrets/RedactPrivate
// a título y contenido, y hacerlo dos veces solo añadiría una ruta que puede
// divergir de la que de verdad protege la escritura.
func (l ReviewLearning) Memory(project string) (Memory, error) {
	if err := l.validar(); err != nil {
		return Memory{}, err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**Problema** (%s", strings.TrimSpace(l.Category))
	if c := strings.TrimSpace(l.Component); c != "" {
		fmt.Fprintf(&sb, " · %s", c)
	}
	fmt.Fprintf(&sb, "): %s\n\n", strings.TrimSpace(l.Problem))
	fmt.Fprintf(&sb, "**Causa raíz**: %s\n\n", strings.TrimSpace(l.RootCause))
	fmt.Fprintf(&sb, "**Resolución**: %s\n", strings.TrimSpace(l.Resolution))

	if verificaciones := limpiarLista(l.Verification); len(verificaciones) > 0 {
		sb.WriteString("\n**Verificación**:\n")
		for _, v := range verificaciones {
			fmt.Fprintf(&sb, "- %s\n", v)
		}
	}
	if c := strings.TrimSpace(l.Confidence); c != "" {
		fmt.Fprintf(&sb, "\n**Confianza**: %s\n", c)
	}
	fmt.Fprintf(&sb, "\nOrigen: revisión adversarial %s.\n", strings.TrimSpace(l.ReviewID))

	titulo := fmt.Sprintf("Revisión: %s en %s", strings.TrimSpace(l.Category), strings.TrimSpace(l.Component))
	if strings.TrimSpace(l.Component) == "" {
		titulo = fmt.Sprintf("Revisión: %s", strings.TrimSpace(l.Category))
	}

	return Memory{
		Project:  project,
		Type:     Learning,
		Title:    titulo,
		Content:  sb.String(),
		TopicKey: l.TopicKey(),
	}, nil
}

func (l ReviewLearning) validar() error {
	faltan := []string{}
	for campo, valor := range map[string]string{
		"categoría":  l.Category,
		"problema":   l.Problem,
		"causa raíz": l.RootCause,
		"resolución": l.Resolution,
	} {
		if strings.TrimSpace(valor) == "" {
			faltan = append(faltan, campo)
		}
	}
	if len(faltan) == 0 {
		return nil
	}
	// Orden estable: el recorrido de un mapa no lo tiene, y un mensaje de error
	// que cambia entre ejecuciones idénticas es imposible de testear.
	sort.Strings(faltan)
	return fmt.Errorf(
		"un aprendizaje de revisión sin %s no es conocimiento reutilizable: no se promueve",
		strings.Join(faltan, ", "),
	)
}

func limpiarLista(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// PromotableFindings devuelve los identificadores locales de los hallazgos que
// cumplen la condición de promoción: CONFIRMED y resuelto EN la ronda indicada
// (FR-033).
//
// La ronda es parte de la condición, no un adorno. Comprobar solo RESOLVED aplicaba
// a este dato un criterio distinto del que aplica el veredicto: el cierre trataba la
// verificación de una ronda pasada como no vigente —falla cerrado— y esta consulta la
// seguía anunciando como promovible —falla abierto—, sobre exactamente la misma fila.
//
// Es una consulta, no una acción, y esa distinción es deliberada. gomemory no
// puede promover por su cuenta porque no puede inventar el conocimiento: el
// problema, la causa raíz y la resolución los redacta quien revisó. Lo que sí
// puede —y debe— es decir con autoridad CUÁLES tienen derecho a producirlo, para
// que la promoción no dependa de que el agente recuerde la regla.
func PromotableFindings(consensus []ConsensusFinding, round int) []string {
	out := make([]string, 0, len(consensus))
	for _, finding := range consensus {
		if finding.Status == ConsensusConfirmed && finding.ResueltoEn(round) {
			out = append(out, finding.ConsensusLocalID)
		}
	}
	sort.Strings(out)
	return out
}
