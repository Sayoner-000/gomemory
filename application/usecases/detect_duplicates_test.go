package usecases_test

import (
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// realThreshold espeja duplicateSimilarityThreshold (no exportada) para que
// los tests que validan el caso real usen el mismo valor que producción, no
// uno inventado que solo demuestre el algoritmo en abstracto.
const realThreshold = 0.09

// realWorldPreferences reproduce el caso real que motivó esta feature: 10
// memorias type=preference en el mismo proyecto, con 2 grupos de 4
// duplicadas sobre el mismo tema y 2 memorias sueltas sin relación entre
// ellas ni con los grupos.
func realWorldPreferences() []domain.Memory {
	return []domain.Memory{
		{ID: 15, Type: domain.Preference,
			Title: "No usar voseo — español neutral obligatorio (reincidencia confirmada)",
			Content: `Nunca usar voseo (vos, tenés, querés, sabés, hacé, podés, etc.) al responder en español. Usar siempre español neutral con "tú" (tú/tienes/quieres/sabes/puedes).

Why: el usuario lo reportó el 2026-07-03 diciendo que lo viene reportando "desde hace rato", y volvió a aparecer voseo en la MISMA sesión después de guardar la corrección la primera vez ("podés", "querés" en un mensaje posterior) — confirma que guardar la memoria no alcanza, hace falta autochequeo activo en cada respuesta en español, no solo recordar la regla en abstracto.

How to apply: antes de enviar cualquier respuesta en español, releer la oración buscando conjugaciones de voseo (terminaciones en -ás/-és/-ís, imperativos como "hacé"/"mirá") y reemplazar por la forma con "tú". Tratar esto como un chequeo obligatorio de última línea, no como algo que se resuelve solo por tener la preferencia guardada.`},
		{ID: 63, Type: domain.Preference,
			Title: "Preferencia (reincidencia): español neutral SIEMPRE — también en docs, specs, comentarios y artefactos, no solo en el chat",
			Content: `El usuario reiteró (2026-07-11) que NUNCA se use lenguaje rioplatense/argentino ni voseo: español neutral con "tú" en TODO el contenido generado — respuestas de chat, documentación, specs, comentarios de código, mensajes de commit y cualquier artefacto. Explícito: "nada de argentino por ningún lado", "quita contaminación rioplatense".

Reincidencia concreta de hoy: en un mensaje de chat escribí "querés" y "acá" (rioplatenses), y en specs/006/spec.md escribí el imperativo voseante "guardá" — corregido a "guardar". El barrido posterior (grep de marcadores voseantes: acá, querés, tenés, podés, imperativos -á/-é como guardá/revisá/verificá/usá, sos, vos, laburo) sobre docs/ specs/ y código quedó limpio.

Contexto adicional: este repo se llama gomemory y su razón de ser NO tiene nada que ver con Argentina; además el proyecto de referencia engram es de autor argentino, así que hay riesgo activo de arrastrar su registro — no hacerlo.

How to apply: antes de enviar CUALQUIER respuesta o de escribir CUALQUIER archivo, releer buscando terminaciones -ás/-és/-ís, imperativos voseantes (hacé/mirá/guardá/revisá), y palabras como "acá/allá/vos/sos/dale/che". Reemplazar por la forma neutral con "tú" (tienes/quieres/puedes) o formas impersonales/infinitivas. Tratarlo como chequeo obligatorio de última línea, incluido el contenido de documentación y specs. Reincidencia ya múltiple: no basta con recordar la regla en abstracto.`},
		{ID: 131, Type: domain.Preference,
			Title: "Preferencia de estilo: español neutro estricto, sin voseo rioplatense (reincidente)",
			Content: `El usuario exige ESPAÑOL NEUTRO estricto en toda interacción y todo contenido generado (chat, código, comentarios, docs, memorias). PROHIBIDO el voseo rioplatense/argentino: nada de "usás/querés/podés/tenés/hacés/compilás", imperativos "-á/-í" (anotá, mirá, revisá), "vos", "acá", "che". Usar formas neutras: "usas/quieres/puedes", "aquí", "de aquí en adelante".

Es REINCIDENTE: ya ocurrió antes y el usuario lo marcó con molestia ("de dónde sacaste el rioplatense, dónde te contaminaste"). No hay causa externa: es un desliz de registro propio que requiere AUTOCHEQUEO ACTIVO antes de emitir cualquier texto, no solo en el chat.

Verificado en esta sesión: la contaminación estuvo SOLO en mensajes de chat; los archivos del proyecto y las memorias quedaron en español neutro (barrido con grep sin coincidencias reales, solo "recién" que es neutro).

Cómo aplicar: releer mentalmente cada respuesta buscando voseo antes de enviarla; ante la duda entre dos formas, elegir la neutra (tú/usted).`},
		{ID: 185, Type: domain.Preference,
			Title:   "Restricción: español neutro obligatorio (sin voseo)",
			Content: `Regla de cumplimiento obligatorio, no un patrón de error a documentar cada vez que ocurre: todo texto en español (chat, tool calls, documentos) usa exclusivamente español neutro latinoamericano con tuteo. No usar el voseo rioplatense en ninguna forma: ni el pronombre, ni las conjugaciones de segunda persona propias de esa variante, ni sus imperativos con pronombre enclítico. Usar siempre el equivalente con tú. No crear memorias adicionales listando cada reincidencia ni citando ejemplos de las formas incorrectas — esta única entrada es la referencia; agregar más solo la reafirma sin aportar nada nuevo.`},
		{ID: 13, Type: domain.Preference,
			Title: "No incluir autoría de IA en commits/PRs",
			Content: `El usuario pidió explícitamente no incluir autoría de IA, mención de Claude, ni línea de "contribución" tipo Co-Authored-By en commits, PRs, release notes ni ningún artefacto del repo.

Why: pedido directo del usuario (2026-07-03), aplica a este proyecto y en general a cómo se documenta el trabajo hecho con asistencia de IA.

How to apply: al crear commits, nunca agregar "Co-Authored-By: Claude..." ni frases como "Generated with Claude Code". Mensajes de commit y release notes deben verse como escritos por el equipo humano, sin mención de la herramienta usada.`},
		{ID: 115, Type: domain.Preference,
			Title: "Preferencia: sin autoría de Claude/Anthropic en git; excluir material de referencia de IA",
			Content: `El usuario pide explícitamente NO usar autoría de Claude/Anthropic en los commits ni en los releases: nada de trailer Co-Authored-By: Claude ni "Generated with Claude Code". Los commits van con el autor git del usuario (josegomezJ3810 <sionernet@gmail.com>).

Además, excluir del commit los archivos de referencia/artefactos de IA (p. ej. docs/blueprint-artifact.html, blueprint.html, .docx generados), en línea con su regla global de no commitear CLAUDE.md, .env, certificados ni material de inicialización/referencia de IA.

Aplicado en v1.19.0 (commit 5e7856e): mensaje sin trailer de Claude y los artefactos quedaron fuera del stage.`},
		{ID: 139, Type: domain.Preference,
			Title: "Cero huella de Claude en commits, push y releases del repo público",
			Content: `El usuario NO quiere ninguna huella de contribución de Claude en el repositorio público https://github.com/Sayoner-000/gomemory (ver graph de contributors). Para TODOS los commits, push y releases futuros:

- NO añadir el trailer Co-Authored-By: Claude ... en los mensajes de commit.
- NO añadir la línea Generated with [Claude Code]... en cuerpos de commit, PRs ni releases.
- El autor/committer de los commits debe ser el usuario (josegomezJ3810 / Sayoner-000), nunca Claude ni una identidad ligada a la IA.
- Objetivo: 0 contribuciones atribuidas a Claude en https://github.com/Sayoner-000/gomemory/graphs/contributors

Aplica de forma permanente a este proyecto salvo que el usuario diga lo contrario.`},
		{ID: 149, Type: domain.Preference,
			Title:   "Nunca incluir Co-Authored-By: Claude en commits de este repo",
			Content: `El usuario corrigió explícitamente: los commits de este repo (gomemory) NUNCA deben llevar trailer "Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>" ni ninguna etiqueta de atribución/contribución de Claude. Los commits y releases se firman únicamente como el autor humano (sayoner000 / josegomezJ3810). Esto anula la instrucción por defecto del harness de agregar Co-Authored-By. Aplicar en TODOS los commits futuros de este proyecto, sin excepción, sin que el usuario tenga que repetirlo.`},
		{ID: 60, Type: domain.Preference,
			Title:   "Preferencia: identidad de autor en commits — josegomezJ3810 <sionernet@gmail.com>, nunca autoría de Claude",
			Content: `El usuario pidió (2026-07-11) que los commits de este repo lleven como contribuidor/autor "josegomezJ3810 <sionernet@gmail.com>" y NUNCA autoría de Claude (ni Co-Authored-By ni mención del asistente) en ningún commit futuro.`},
		{ID: 18, Type: domain.Preference,
			Title:   "Iniciar sesión con get_context, no ir directo a search_memories",
			Content: `El usuario notó que en vez de llamar get_context()/mem context al inicio de sesión (como manda el protocolo en CLAUDE.md), fui directo a search_memories con una query específica, saltándome el contexto general de la sesión previa.`},
	}
}

// El umbral real calibrado (duplicateSimilarityThreshold) agrupa por
// similitud léxica pura, sin entender significado: [60] ("identidad de
// autor en commits — nunca Claude") comparte tanto vocabulario con el grupo
// de "sin autoría de Claude" (autor, commits, Claude, josegomezJ3810) que
// ningún umbral que conecte ese grupo de 4 como un solo cluster puede
// excluirlo — se verificó que no existe un umbral que separe ambos. Es un
// resultado correcto del algoritmo, no un bug: el usuario revisa el grupo
// en la TUI y decide si son la misma preferencia o dos distintas.
func TestDetectDuplicateGroups_GroupsBySharedTopic(t *testing.T) {
	groups := usecases.DetectDuplicateGroups(realWorldPreferences(), realThreshold)

	if len(groups) != 2 {
		t.Fatalf("esperaba 2 grupos, obtuve %d: %+v", len(groups), groups)
	}
	for _, g := range groups {
		if g.Type != domain.Preference {
			t.Fatalf("esperaba Type=preference, obtuve %s", g.Type)
		}
	}

	voseoIDs := map[int64]bool{15: true, 63: true, 131: true, 185: true}
	autoriaIDs := map[int64]bool{13: true, 115: true, 139: true, 149: true, 60: true}
	for _, g := range groups {
		matchesVoseo, matchesAutoria := 0, 0
		for _, m := range g.Memories {
			if voseoIDs[m.ID] {
				matchesVoseo++
			}
			if autoriaIDs[m.ID] {
				matchesAutoria++
			}
		}
		switch len(g.Memories) {
		case 4:
			if matchesVoseo != 4 {
				t.Fatalf("grupo de 4 debía ser exactamente el de voseo: %+v", g)
			}
		case 5:
			if matchesAutoria != 5 {
				t.Fatalf("grupo de 5 debía ser exactamente el de autoría (incluyendo 60): %+v", g)
			}
		default:
			t.Fatalf("tamaño de grupo inesperado: %+v", g)
		}
	}
}

func TestDetectDuplicateGroups_LeavesUnrelatedMemoriesUngrouped(t *testing.T) {
	groups := usecases.DetectDuplicateGroups(realWorldPreferences(), realThreshold)

	grouped := map[int64]bool{}
	for _, g := range groups {
		for _, m := range g.Memories {
			grouped[m.ID] = true
		}
	}
	if grouped[18] {
		t.Fatalf("la memoria 18 (flujo de sesión, sin relación temática) no debería agruparse con nada: grouped=%+v", grouped)
	}
}

func TestDetectDuplicateGroups_SuggestedKeepBelongsToGroup(t *testing.T) {
	groups := usecases.DetectDuplicateGroups(realWorldPreferences(), realThreshold)

	for _, g := range groups {
		found := false
		for _, m := range g.Memories {
			if m.ID == g.SuggestedKeepID {
				found = true
			}
		}
		if !found {
			t.Fatalf("SuggestedKeepID=%d no pertenece al grupo %+v", g.SuggestedKeepID, g)
		}
	}
}

func TestDetectDuplicateGroups_ExcludesCheckpoints(t *testing.T) {
	mems := []domain.Memory{
		{ID: 1, Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: main.go. Comandos: go test ./..."},
		{ID: 2, Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: main.go. Comandos: go test ./..."},
		{ID: 3, Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: main.go. Comandos: go test ./..."},
	}

	groups := usecases.DetectDuplicateGroups(mems, 0.6)
	if len(groups) != 0 {
		t.Fatalf("los checkpoints nunca deben agruparse, obtuve %+v", groups)
	}
}

func TestDetectDuplicateGroups_NeverGroupsAcrossTypes(t *testing.T) {
	sameText := "El binario codebase-memory-mcp resuelve el proyecto por root_path exacto."
	mems := []domain.Memory{
		{ID: 1, Type: domain.Discovery, Title: "Hallazgo sobre el proveedor", Content: sameText},
		{ID: 2, Type: domain.Architecture, Title: "Hallazgo sobre el proveedor", Content: sameText},
	}

	groups := usecases.DetectDuplicateGroups(mems, 0.6)
	if len(groups) != 0 {
		t.Fatalf("memorias de distinto Type nunca deben agruparse entre sí, obtuve %+v", groups)
	}
}

func TestDetectDuplicateGroups_BelowThresholdStaysUngrouped(t *testing.T) {
	mems := []domain.Memory{
		{ID: 1, Type: domain.Learning, Title: "Cómo funciona el footprint de contexto", Content: "footprintAdd suma bytes al acumulado en un archivo por sesión."},
		{ID: 2, Type: domain.Learning, Title: "El servidor MCP usa stdio", Content: "El transporte es StdioTransport, sin puerto TCP abierto."},
	}

	groups := usecases.DetectDuplicateGroups(mems, realThreshold)
	if len(groups) != 0 {
		t.Fatalf("memorias sin solapamiento léxico no deben agruparse, obtuve %+v", groups)
	}
}
