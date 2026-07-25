package usecases

import (
	"regexp"
	"sort"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// duplicateSimilarityThreshold es el Jaccard mínimo (tras filtrar
// stopwords) entre tokens de dos memorias del mismo Type para considerarlas
// candidatas al mismo grupo. Calibrado contra un caso real (10 memorias
// type=preference, 2 grupos de duplicados genuinos): sobre prosa libre en
// español, incluso duplicados claros rondan Jaccard 0.08-0.30 (mucho
// vocabulario único por narración distinta del mismo incidente) — un umbral
// alto tipo 0.6 nunca dispara. Deliberadamente bajo y orientado a recall: es
// un candidato para revisión humana en la TUI, no un borrado automático, así
// que un falso positivo cuesta un vistazo y "esc"; un falso negativo es una
// duplicidad que nunca se sugiere.
// minTokenLength descarta tokens de 1 letra (ruido: preposiciones sueltas,
// iniciales) que no aportan señal de tema compartido.
const (
	duplicateSimilarityThreshold = 0.09
	minTokenLength               = 2
)

var tokenPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

// spanishStopwords son palabras funcionales de altísima frecuencia que
// aparecen en casi cualquier par de memorias en español y diluyen la señal
// de Jaccard si no se filtran (dos memorias sin relación alguna ya comparten
// "el", "de", "que", "usuario" solo por estar escritas en el mismo idioma).
// Lista acotada a artículos/preposiciones/conjunciones/pronombres core, sin
// intentar cubrir conjugaciones verbales (esas sí son señal: la conjugación
// concreta que usa cada memoria importa para agrupar por tema).
var spanishStopwords = map[string]struct{}{
	"el": {}, "la": {}, "los": {}, "las": {}, "un": {}, "una": {}, "unos": {}, "unas": {},
	"de": {}, "del": {}, "al": {}, "a": {}, "en": {}, "con": {}, "sin": {}, "por": {}, "para": {},
	"y": {}, "o": {}, "ni": {}, "que": {}, "como": {}, "más": {}, "pero": {}, "si": {},
	"su": {}, "sus": {}, "se": {}, "lo": {}, "le": {}, "les": {}, "es": {}, "son": {}, "ser": {},
	"este": {}, "esta": {}, "esto": {}, "estos": {}, "estas": {}, "ese": {}, "esa": {}, "eso": {},
	"no": {}, "ya": {}, "muy": {}, "también": {}, "solo": {}, "sólo": {}, "aquí": {}, "así": {},
	"todo": {}, "toda": {}, "todos": {}, "todas": {}, "cada": {}, "otro": {}, "otra": {},
	"cuando": {}, "donde": {}, "porque": {}, "sobre": {}, "entre": {}, "hasta": {}, "desde": {},
	"me": {}, "mi": {}, "tu": {}, "él": {}, "ella": {}, "nos": {}, "vez": {}, "ha": {}, "he": {},
}

// DuplicateGroup es un conjunto de memorias del mismo Type que la detección
// considera probablemente redundantes entre sí (mismo tema, contenido
// solapado). SuggestedKeepID es una sugerencia de cuál conservar, no una
// decisión final — la TUI la usa como default preseleccionado, el usuario
// puede cambiarla antes de confirmar el borrado del resto.
type DuplicateGroup struct {
	Type            domain.MemoryType
	Memories        []domain.Memory
	SuggestedKeepID int64
}

// DetectDuplicateGroups agrupa memorias probablemente redundantes dentro de
// un mismo Type (nunca cross-type) usando similitud léxica Jaccard sobre
// tokens de Title+Content, clusterizadas por union-find. Pura y testeable:
// no toca DB ni ningún puerto — opera sobre memorias ya cargadas (p. ej. vía
// MemoryRepository.ListAll). threshold es explícito para poder probar
// distintos valores en tests; los llamadores de producción usan
// duplicateSimilarityThreshold.
//
// Checkpoint queda siempre excluido: es un log automático de actividad por
// turno, no conocimiento curado, y su contenido se repite por diseño (mismos
// archivos tocados en turnos consecutivos) sin que eso sea una duplicidad a
// resolver.
// DetectProjectDuplicates carga todas las memorias del proyecto (vía
// MemoryRepository.ListAll, sin el tope de List) y les aplica
// DetectDuplicateGroups con el umbral de producción. Mismo patrón que
// ExportProject (portability.go): el usecase envuelve el repo en vez de que
// cada llamador (TUI, futura tool MCP) repita la carga + el umbral.
func DetectProjectDuplicates(memRepo ports.MemoryRepository, project string) ([]DuplicateGroup, error) {
	mems, err := memRepo.ListAll(project)
	if err != nil {
		return nil, err
	}
	return DetectDuplicateGroups(mems, duplicateSimilarityThreshold), nil
}

func DetectDuplicateGroups(memories []domain.Memory, threshold float64) []DuplicateGroup {
	byType := make(map[domain.MemoryType][]domain.Memory)
	for _, m := range memories {
		if m.Type == domain.Checkpoint {
			continue
		}
		byType[m.Type] = append(byType[m.Type], m)
	}

	var groups []DuplicateGroup
	for t, mems := range byType {
		for _, cluster := range clusterBySimilarity(mems, threshold) {
			if len(cluster) < 2 {
				continue
			}
			sort.Slice(cluster, func(i, j int) bool { return cluster[i].ID < cluster[j].ID })
			groups = append(groups, DuplicateGroup{
				Type:            t,
				Memories:        cluster,
				SuggestedKeepID: suggestKeep(cluster),
			})
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Type != groups[j].Type {
			return groups[i].Type < groups[j].Type
		}
		return groups[i].Memories[0].ID < groups[j].Memories[0].ID
	})
	return groups
}

// clusterBySimilarity agrupa memorias por componentes conexas de un grafo
// donde dos memorias están unidas si su similitud supera threshold. La
// transitividad importa: en el caso real que motivó esta función, 4 memorias
// sobre el mismo tema no son necesariamente similares 2 a 2 con el mismo
// umbral, pero forman una cadena (A~B~C~D) que union-find resuelve como un
// solo cluster.
func clusterBySimilarity(mems []domain.Memory, threshold float64) [][]domain.Memory {
	n := len(mems)
	tokens := make([]map[string]struct{}, n)
	for i, m := range mems {
		tokens[i] = tokenize(m.Title + " " + m.Content)
	}

	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if jaccardSimilarity(tokens[i], tokens[j]) >= threshold {
				union(i, j)
			}
		}
	}

	clusters := make(map[int][]domain.Memory)
	for i, m := range mems {
		root := find(i)
		clusters[root] = append(clusters[root], m)
	}

	result := make([][]domain.Memory, 0, len(clusters))
	for _, c := range clusters {
		result = append(result, c)
	}
	return result
}

// tokenize normaliza texto a un set de palabras en minúscula, sin stemming
// ni stopwords (mantiene la implementación simple — el umbral de similitud
// ya filtra el ruido de coincidencias cortas). Descarta tokens con menos de
// minTokenLength runas.
func tokenize(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, tok := range tokenPattern.FindAllString(strings.ToLower(s), -1) {
		if len([]rune(tok)) < minTokenLength {
			continue
		}
		if _, stop := spanishStopwords[tok]; stop {
			continue
		}
		set[tok] = struct{}{}
	}
	return set
}

// jaccardSimilarity es |A∩B| / |A∪B|. Dos sets vacíos se consideran sin
// similitud (0), no indefinidos, para no formar clusters de memorias sin
// texto útil.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for tok := range a {
		if _, ok := b[tok]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// suggestKeep prioriza el contenido más completo (Content más largo) como
// sugerencia de cuál conservar; en empate, la más reciente por CreatedAt
// (formato ISO, comparable como string). Es solo un default preseleccionado
// en la TUI, no una decisión automática.
func suggestKeep(group []domain.Memory) int64 {
	best := group[0]
	for _, m := range group[1:] {
		switch {
		case len(m.Content) > len(best.Content):
			best = m
		case len(m.Content) == len(best.Content) && m.CreatedAt > best.CreatedAt:
			best = m
		}
	}
	return best.ID
}
