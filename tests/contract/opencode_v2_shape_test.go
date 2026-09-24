package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestOpenCodeV2_ExportDefaultDual cubre FR-001 y FR-002 de la feature 032.
//
// OpenCode 2.x valida el export `default` y exige { id, setup }; sin él el
// plugin falla al cargar ("Plugin must export a default definition…") y todas
// las capacidades desaparecen. OpenCode 1.x (verificado desde 1.17.0) llama a
// `server` del mismo objeto. El export con nombre se conserva como red para
// cargadores 1.x más antiguos y para las pruebas del plugin.
func TestOpenCodeV2_ExportDefaultDual(t *testing.T) {
	texto := gomemoryPluginSource(t)
	for _, requerido := range []string{
		"export const GomemoryPlugin",
		"export default {",
		`id: "gomemory"`,
		"setup: createV2Setup(",
		"server: GomemoryPlugin",
	} {
		if !strings.Contains(texto, requerido) {
			t.Errorf("el plugin no contiene %q: OpenCode 2.x o 1.x dejaría de cargarlo", requerido)
		}
	}
}

// TestOpenCodeV2_SinImportsDeRuntimeAjenos: el plugin se instala como archivo
// suelto en ~/.config/opencode/plugins/, sin node_modules. Un import de
// runtime que no sea de node:* falla al cargar en la máquina de la persona
// usuaria aunque pase aquí. `import type` se borra al ejecutar y sí se permite.
func TestOpenCodeV2_SinImportsDeRuntimeAjenos(t *testing.T) {
	texto := gomemoryPluginSource(t)
	re := regexp.MustCompile(`(?m)^import\s+(type\s+)?[^;]*?from\s+"([^"]+)"`)
	encontrados := re.FindAllStringSubmatch(texto, -1)
	if len(encontrados) == 0 {
		t.Fatal("no se detectó ningún import: el extractor dejó de reconocer la forma del plugin")
	}
	for _, m := range encontrados {
		if m[1] != "" {
			continue
		}
		if !strings.HasPrefix(m[2], "node:") {
			t.Errorf("import de runtime %q: el plugin instalado no tiene node_modules y fallaría al cargar", m[2])
		}
	}
}

// v2Block recorta el bloque de un gancho v2: desde su marcador `// v2:<nombre>`
// hasta el siguiente marcador `// v2:`. Es el equivalente v2 de blockBetween,
// que depende de la sangría de las claves del objeto de ganchos v1.
func v2Block(t *testing.T, texto, nombre string) string {
	t.Helper()
	marcador := "// v2:" + nombre + "\n"
	i := strings.Index(texto, marcador)
	if i < 0 {
		t.Fatalf("no se encontró el marcador %q en el plugin", strings.TrimSpace(marcador))
	}
	resto := texto[i+len(marcador):]
	if j := strings.Index(resto, "// v2:"); j >= 0 {
		resto = resto[:j]
	}
	return resto
}

// TestOpenCodeV2_GanchosRegistrados cubre US1 y US3: cada capacidad v2 se
// engancha al dominio correcto y delega en el gancho v1 que ya tiene la lógica.
func TestOpenCodeV2_GanchosRegistrados(t *testing.T) {
	texto := gomemoryPluginSource(t)
	casos := []struct{ bloque, registro, delega string }{
		{"event", "ctx.event.subscribe(", "hooks.event("},
		{"prompt", `ctx.session.hook("prompt"`, `hooks["chat.message"]`},
		{"context", `ctx.session.hook("context"`, `hooks["experimental.chat.system.transform"]`},
		{"compaction", `ctx.session.hook("compaction"`, `hooks["experimental.session.compacting"]`},
		{"tool.execute.after", `ctx.tool.hook("execute.after"`, `hooks["tool.execute.after"]`},
		{"cleanup", "controller.abort()", "hooks.dispose()"},
	}
	for _, c := range casos {
		b := v2Block(t, texto, c.bloque)
		if !strings.Contains(b, c.registro) {
			t.Errorf("bloque v2:%s no registra %q", c.bloque, c.registro)
		}
		if !strings.Contains(b, c.delega) {
			t.Errorf("bloque v2:%s no delega en %q: la capacidad se perdería en OpenCode 2.x", c.bloque, c.delega)
		}
	}
}

// TestOpenCodeV2_EventosTraducidos: los nombres de evento v2 que alimentan la
// sesión, el checkpoint y la recuperación post-compactación (research R-5).
// Y el filtro por directorio: v2 entrega eventos de todas las ubicaciones.
func TestOpenCodeV2_EventosTraducidos(t *testing.T) {
	b := v2Block(t, gomemoryPluginSource(t), "event")
	for _, par := range [][2]string{
		{`"session.created"`, `"session.created"`},
		{`"session.idle"`, `"session.idle"`},
		{`"session.deleted"`, `"session.deleted"`},
		{`"session.compaction.ended"`, `"session.compacted"`},
		{`"session.execution.succeeded"`, `"session.idle"`},
	} {
		if !strings.Contains(b, par[0]+": "+par[1]) {
			t.Errorf("el bloque v2:event no traduce %s → %s", par[0], par[1])
		}
	}
	if !strings.Contains(b, "ownsSession(sessionID, hinted)") {
		t.Error("el bloque v2:event no comprueba el dueño de la sesión: otra ubicación dispararía sesiones y checkpoints de este proyecto")
	}
}

// TestOpenCodeV2_RamaV1ConservaGanchos cubre FR-002: la rama v1 sigue
// registrando exactamente los mismos ganchos que antes de la feature 032.
func TestOpenCodeV2_RamaV1ConservaGanchos(t *testing.T) {
	got := strings.Join(hooksQueRegistraElComplemento(t), ",")
	want := "chat.message,experimental.chat.system.transform,experimental.session.compacting,tool.execute.after"
	if got != want {
		t.Errorf("ganchos v1 = %s; want %s", got, want)
	}
	if !strings.Contains(gomemoryPluginSource(t), "    event: async ({ event }) =>") {
		t.Error("la rama v1 perdió su gancho event")
	}
}

// TestOpenCodeV2_FabricaV1IdenticaALaReferencia cubre FR-002 con la garantía
// más fuerte posible: la fábrica que ejecuta OpenCode 1.x es, texto a texto,
// la misma que antes de la feature 032 (copia en specs/…/baseline). La rama v2
// se añadió aparte; si alguien toca la fábrica, este contrato obliga a
// decidirlo a conciencia y actualizar la referencia.
func TestOpenCodeV2_FabricaV1IdenticaALaReferencia(t *testing.T) {
	ref, err := os.ReadFile("../../specs/032-opencode-v2-plugin-compat/baseline/gomemory.v1.ts")
	if err != nil {
		t.Skipf("sin copia de referencia v1: %v", err)
	}
	fabrica := func(texto string) string {
		i := strings.Index(texto, "export const GomemoryPlugin")
		if i < 0 {
			t.Fatal("no se encontró la fábrica GomemoryPlugin")
		}
		j := strings.Index(texto[i:], "\n};\n")
		if j < 0 {
			t.Fatal("no se encontró el cierre de la fábrica GomemoryPlugin")
		}
		return texto[i : i+j]
	}
	if fabrica(gomemoryPluginSource(t)) != fabrica(string(ref)) {
		t.Error("la fábrica v1 cambió respecto de la referencia: OpenCode 1.x dejaría de comportarse igual (FR-002)")
	}
}

// TestOpenCodeV2_GanchosFiltranPorDueñoDeSesion: el servicio 2.x entrega los
// ganchos de sesión a todas las instancias (reproducido con dos proyectos).
// Cada gancho que llama a mem debe comprobar primero que la sesión es suya.
func TestOpenCodeV2_GanchosFiltranPorDueñoDeSesion(t *testing.T) {
	texto := gomemoryPluginSource(t)
	for _, nombre := range []string{"prompt", "context", "compaction", "tool.execute.after"} {
		if !strings.Contains(v2Block(t, texto, nombre), "ownsSession(ev.sessionID)") {
			t.Errorf("el gancho v2:%s no comprueba el dueño de la sesión: inyectaría o registraría en el proyecto equivocado", nombre)
		}
	}
}
