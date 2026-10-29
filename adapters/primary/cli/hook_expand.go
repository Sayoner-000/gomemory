package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// expandStdinMax es el tope del payload que lee `mem hook expand`. Los
// campos no se recortan por separado: una regla debe ver el comando, el
// contenido o el prompt completos, o un relleno delante la eludiría (B-11
// del ACR). Un pre-tool de shell mayor que el tope se bloquea
// (oversizedShellPayload).
const expandStdinMax = 8 << 20

// hookExpandDeps son las dependencias de runHookExpand, inyectables para
// probar el pánico y el plazo sin disparadores en el binario (2.º analyze,
// K1): las pruebas sustituyen run o los evaluadores.
type hookExpandDeps struct {
	root     string
	settings ports.SettingsData
	// oversized: el payload superó expandStdinMax y llega truncado.
	oversized bool
	hook      usecases.ExpandHookDeps
	// configHash y lastShieldHash solo se consultan en session-start.
	configHash     func(context.Context) string
	lastShieldHash func(context.Context) string
	// record registra una activación (kind, detail); abre la base de forma
	// perezosa. nil = no se registra.
	record func(agent, kind, detail string)
	now    func() time.Time
	run    func(context.Context, domain.ExpandSignals, []domain.SupertoolAction, usecases.ExpandHookDeps) domain.ExpandOutcome
	// hcdActive informa si hay una unidad HCD activa (aviso pre-edición,
	// feature 045 FR-025). nil = HCD no conectado: no se avisa.
	hcdActive func(context.Context) (bool, error)
}

// hookExpand implementa `mem hook expand <event> [--emit=<dialecto>]`
// (feature 040, contracts/hook-expand.md). Regla de oro de CmdHook: nunca
// rompe el turno; ante cualquier fallo, permite sin emitir nada.
func hookExpand(deps *Deps, args []string) {
	if len(args) == 0 {
		os.Exit(0)
	}
	root := deps.Root
	if root == "" && deps.ProjectRepo != nil {
		root, _ = deps.ProjectRepo.FindRoot()
	}
	if root == "" {
		os.Exit(0)
	}
	stdin, _ := io.ReadAll(io.LimitReader(os.Stdin, expandStdinMax+1))
	oversized := len(stdin) > expandStdinMax
	if oversized {
		stdin = stdin[:expandStdinMax]
	}
	ctx, cancel := context.WithTimeout(context.Background(), domain.HookBudget)
	defer cancel()

	// Las activaciones se acumulan y se registran DESPUÉS de escribir la
	// decisión: abrir la base para registrar nunca debe consumir el plazo de
	// la decisión ni descartarla.
	records := &expandRecords{}
	d := hookExpandDeps{root: root, now: time.Now, record: records.add, oversized: oversized}
	if deps.SettingsRepo != nil {
		d.settings = deps.SettingsRepo.Read(root)
	}
	if deps.ShieldState != nil {
		d.lastShieldHash = deps.ShieldState.LastHash
	}
	if deps.AgentConfig != nil {
		d.configHash = deps.AgentConfig.Fingerprint
	}
	d.hook = expandEvaluators(deps, root)
	if deps.HCD != nil {
		d.hcdActive = deps.HCD.HasActive
	}
	// El plegado de instincts es mantenimiento: dentro del plazo solo se
	// anota la petición y se ejecuta tras escribir la decisión (H-01).
	fold := deferFold(&d.hook)

	emit := emitFlagValue(args)
	out, ok := expandWithBudget(domain.HookBudget, func() hookRenderedOutput {
		return runHookExpand(ctx, d, args[0], emit, stdin)
	})
	if !ok {
		// El plazo venció fuera de lo que ctx pudo cortar: allow silencioso.
		payload, _ := parseHookPayload(stdin)
		dialect := detectDialect(payload, emit)
		if ev, valid := domain.ParseExpandEvent(args[0]); valid {
			guard, _ := builtinShellGuard(d.settings, ev, payload, stdin)
			out = renderExpandOutput(dialect, ev, guard)
		}
		if d.record != nil {
			d.record(agentOfDialect(dialect), "expand.timeout", args[0])
		}
	}
	if out.stdout != "" {
		_, _ = os.Stdout.WriteString(out.stdout)
	}
	if out.stderr != "" {
		_, _ = os.Stderr.WriteString(out.stderr)
	}
	if fold.requested.Load() {
		payload, _ := parseHookPayload(stdin)
		agent := agentOfDialect(detectDialect(payload, emit))
		runDeferredFold(fold.inner.Fold, expandFoldBudget, func(kind, detail string) {
			records.add(agent, kind, detail)
		})
	}
	records.flush(lazyExpandRecorder(deps), expandRecordBudget)
	os.Exit(out.exitCode)
}

// expandFoldBudget es el plazo propio del plegado de instincts (hasta 500
// observaciones más abrir la base y el store global). Corre con la decisión
// ya escrita, así que no compite con los 150 ms de domain.HookBudget.
const expandFoldBudget = 2 * time.Second

// foldRequest sustituye el Fold de un InstinctObserver por una anotación:
// el router sigue decidiendo cuándo plegar, pero el trabajo sale del plazo.
type foldRequest struct {
	usecases.InstinctObserver
	inner     usecases.InstinctObserver
	requested *atomic.Bool
}

func (f foldRequest) Fold(context.Context) error {
	f.requested.Store(true)
	return nil
}

// deferFold envuelve h.Instincts y devuelve la petición; sin instincts, la
// petición nunca se activa.
func deferFold(h *usecases.ExpandHookDeps) foldRequest {
	req := foldRequest{requested: &atomic.Bool{}}
	if h.Instincts != nil {
		req.InstinctObserver, req.inner = h.Instincts, h.Instincts
		h.Instincts = req
	}
	return req
}

// runDeferredFold pliega con su propio plazo y deja constancia de cómo
// terminó: expand.timeout si venció (la cola se conserva, porque el borrado
// solo ocurre tras confirmar el plegado) o expand.error si falló. La
// goroutine protege del driver SQLite, que no siempre atiende a ctx
// (memoria 552).
func runDeferredFold(fold func(context.Context) error, budget time.Duration, record func(kind, detail string)) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("pánico: %v", r)
			}
		}()
		done <- fold(ctx)
	}()
	var err error
	select {
	case err = <-done:
	case <-time.After(budget + 25*time.Millisecond):
		err = context.DeadlineExceeded
	}
	switch {
	case err == nil:
	case errors.Is(err, context.DeadlineExceeded):
		record("expand.timeout", string(domain.ActionInstinctsFold))
	default:
		record("expand.error", string(domain.ActionInstinctsFold)+": "+domain.StripTerminalControls(domain.ProtectText(err.Error())))
	}
}

// expandRecordBudget acota el registro de activaciones, que ocurre ya con la
// decisión escrita: si la base tarda, se pierde el registro, no la decisión.
// Es el plazo de expandFoldBudget: la primera apertura crea y migra la base
// (0,4 s en un equipo rápido, más en Windows) y con 500 ms el registro de un
// proyecto nuevo se perdía. Con la base creada tarda milisegundos.
const expandRecordBudget = expandFoldBudget

// expandRecords acumula activaciones de forma segura entre la goroutine de
// la decisión (que puede seguir viva tras el plazo) y el proceso principal.
type expandRecords struct {
	mu   sync.Mutex
	list [][3]string
}

func (r *expandRecords) add(agent, kind, detail string) {
	r.mu.Lock()
	r.list = append(r.list, [3]string{agent, kind, detail})
	r.mu.Unlock()
}

func (r *expandRecords) flush(record func(agent, kind, detail string), budget time.Duration) {
	r.mu.Lock()
	list := append([][3]string(nil), r.list...)
	r.mu.Unlock()
	if len(list) == 0 {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		for _, e := range list {
			record(e[0], e[1], e[2])
		}
	}()
	select {
	case <-done:
	case <-time.After(budget):
	}
}

// expandEvaluators enchufa los evaluadores de cada supertool disponibles en
// deps. Cada historia de la feature 040 añade el suyo aquí.
func expandEvaluators(deps *Deps, root string) usecases.ExpandHookDeps {
	var h usecases.ExpandHookDeps
	if deps.RuleSource != nil {
		h.Rules = usecases.ExpandRules{Source: deps.RuleSource, Root: root}
	}
	if deps.AgentConfig != nil {
		h.Shield = usecases.ShieldScanner{Reader: deps.AgentConfig, State: deps.ShieldState}
	}
	if deps.Instincts != nil {
		h.Instincts = deps.Instincts
	} else if deps.ExpandQueue != nil {
		h.Instincts = lazyExpandInstincts{deps}
	}
	return h
}

type lazyExpandInstincts struct{ deps *Deps }

func (l lazyExpandInstincts) Observe(ctx context.Context, sig domain.ExpandSignals) error {
	return (&usecases.Instincts{Queue: l.deps.ExpandQueue}).Observe(ctx, sig)
}
func (l lazyExpandInstincts) Fold(ctx context.Context) error {
	if l.deps.ExpandFull == nil {
		return nil
	}
	// Sin base no hay plegado: es un fallo que debe registrarse, no un «no
	// había nada que plegar» (H-01).
	full, err := l.deps.ExpandFull()
	if err != nil {
		return fmt.Errorf("instincts: no se pudo abrir la base: %w", err)
	}
	if full == nil || full.Instincts == nil {
		return errors.New("instincts: la base abierta no trae el caso de uso")
	}
	return full.Instincts.Fold(ctx)
}

// lazyExpandRecorder devuelve un registrador que solo abre la base (vía
// ExpandFull) la primera vez que hay algo que registrar (2.º analyze, F3).
func lazyExpandRecorder(deps *Deps) func(agent, kind, detail string) {
	var full *Deps
	resolved := false
	return func(agent, kind, detail string) {
		if !resolved {
			resolved = true
			full = deps
			if deps.ChannelActivity == nil && deps.ExpandFull != nil {
				full, _ = deps.ExpandFull()
			}
		}
		recordGuard(full, agent, kind, detail)
	}
}

// expandWithBudget es la red de seguridad de withBudget para un resultado
// renderizado: recupera pánicos y abandona la llamada si vence el plazo
// (ok=false). El corte normal lo hace ctx dentro de runHookExpand.
func expandWithBudget(budget time.Duration, fn func() hookRenderedOutput) (hookRenderedOutput, bool) {
	ch := make(chan hookRenderedOutput, 1)
	go func() {
		defer func() {
			if recover() != nil {
				ch <- hookRenderedOutput{}
			}
		}()
		ch <- fn()
	}()
	select {
	case r := <-ch:
		return r, true
	case <-time.After(budget + 25*time.Millisecond):
		return hookRenderedOutput{}, false
	}
}

// runHookExpand es el núcleo del hook sin E/S de proceso: decide con el
// router una sola vez, ejecuta las acciones, filtra las sugerencias por
// episodio, registra las activaciones y renderiza en el dialecto.
func runHookExpand(ctx context.Context, d hookExpandDeps, event, emit string, stdin []byte) (rendered hookRenderedOutput) {
	ev, ok := domain.ParseExpandEvent(event)
	if !ok {
		return hookRenderedOutput{}
	}
	payload, _ := parseHookPayload(stdin)
	dialect := detectDialect(payload, emit)
	allow := renderExpandOutput(dialect, ev, domain.ExpandOutcome{})
	record := func(kind, detail string) {
		if d.record != nil {
			d.record(agentOfDialect(dialect), kind, detail)
		}
	}
	defer func() {
		if recover() != nil {
			record("expand.error", "panic")
			rendered = allow
		}
	}()
	if d.settings.ExpandDisabled {
		return allow
	}
	if d.oversized && ev == domain.ExpandPreTool && !slices.Contains(d.settings.ExpandOff, string(domain.SupertoolRules)) && oversizedShellPayload(stdin) {
		record("expand.block", "oversized-shell")
		return renderExpandOutput(dialect, ev, domain.ExpandOutcome{Decision: domain.DecisionBlock, Rule: "destructive-shell",
			Reason: "gomemory expand · el comando supera el tamaño que el guardarraíl puede analizar. Si es intencionado, ejecútalo tú en tu terminal o desactiva la regla (mem expand off rules)."})
	}

	in := normalizeExpandPayload(payload, stdin)
	sid := expandSessionKey(in.sessionID)
	sig := domain.ExpandSignals{
		Event:            ev,
		Source:           in.source,
		Tool:             domain.ClassifyTool(in.tool),
		Command:          in.command,
		FilePath:         in.filePath,
		Prompt:           in.prompt,
		CompactThreshold: d.settings.ExpandCompactThreshold,
		Content:          in.content,
		LastMessage:      in.lastMessage,
	}
	switch ev {
	case domain.ExpandPreTool:
		if sig.Tool != domain.ToolOther {
			sig.ToolCount = expandToolCountIncr(d.root, sid)
		}
	case domain.ExpandSessionStart:
		if d.now != nil {
			purgeExpandMarkers(d.root, d.now())
		}
		if d.configHash != nil {
			sig.ConfigHash = d.configHash(ctx)
		}
		if d.lastShieldHash != nil {
			sig.LastShieldHash = d.lastShieldHash(ctx)
		}
	}

	sw := domain.ExpandSwitches{Off: map[domain.SupertoolName]bool{}}
	for _, name := range d.settings.ExpandOff {
		if n, ok := domain.ParseSupertoolName(name); ok {
			sw.Off[n] = true
		}
	}
	actions := domain.RouteSupertools(sig, sw)
	var work []domain.SupertoolAction
	for _, a := range actions {
		switch a.Kind {
		case domain.ActionCounterReset:
			expandToolCountReset(d.root, sid)
		case domain.ActionEpisodeReset:
			expandEpisodeReset(d.root, sid)
		default:
			work = append(work, a)
		}
	}

	run := d.run
	if run == nil {
		run = usecases.ExpandHook{}.Run
	}
	out := run(ctx, sig, work, d.hook)
	if ev == domain.ExpandStop && in.stopHookActive && out.Decision == domain.DecisionBlock {
		// Un solo bloqueo por fin de turno: si el harness ya continuó por uno,
		// se deja terminar (verificado en real: sin esto, una regla stop que
		// siempre coincide bloquea cada intento y el agente entra en bucle).
		// Solo se retira la activación del bloqueo suprimido; los warn y
		// demás ya emitidos se siguen registrando (C-003).
		out.Decision, out.Reason, out.Rule = domain.DecisionAllow, "", ""
		out.Fired = slices.DeleteFunc(out.Fired, func(f string) bool {
			return strings.HasPrefix(f, "expand.block:")
		})
	}
	for _, e := range out.Errors {
		record("expand.error", e)
	}
	if out.TimedOut {
		record("expand.timeout", event)
		if guard, blocked := builtinShellGuard(d.settings, ev, payload, stdin); blocked {
			record("expand.block", guard.Rule)
			return renderExpandOutput(dialect, ev, guard)
		}
		return allow
	}
	if out.Decision != domain.DecisionBlock {
		for _, skill := range out.Suggestions {
			key := "suggest:" + skill
			text := domain.SuggestionText(skill)
			if text == "" || !expandEpisodeClaim(d.root, sid, key) {
				continue
			}
			out.Context = append(out.Context, text)
			out.Fired = append(out.Fired, "expand.suggest:"+skill)
		}
	}
	if out.Decision != domain.DecisionBlock && hcdEditWithoutUnit(ctx, d, sig) && expandEpisodeClaim(d.root, sid, "hcd:no-unit") {
		out.Context = append(out.Context, domain.HCDNoUnitWarning)
		out.Fired = append(out.Fired, "expand.hcd:no-unit")
	}
	for _, f := range out.Fired {
		kind, detail, _ := strings.Cut(f, ":")
		record(kind, detail)
	}
	return renderExpandOutput(dialect, ev, out)
}

// hcdEditWithoutUnit: edición de código sin unidad HCD activa. Ante un error
// de lectura no avisa: el aviso nunca debe estorbar el turno.
func hcdEditWithoutUnit(ctx context.Context, d hookExpandDeps, sig domain.ExpandSignals) bool {
	if d.hcdActive == nil || sig.Event != domain.ExpandPreTool || sig.Tool != domain.ToolFile || !domain.IsHCDCodePath(sig.FilePath) {
		return false
	}
	active, err := d.hcdActive(ctx)
	return err == nil && !active
}

// builtinShellGuard decide un pre-tool de shell solo con la regla de serie
// destructive-shell, que es pura y no lee disco. Se usa cuando vence el plazo
// del hook: sin él, un arranque lento bajo carga dejaba pasar `rm -rf` en
// silencio (B-12 del ACR). Respeta el apagado de expand y de rules; las
// reglas propias (incluida una anulación de la de serie) no se cargan aquí.
func builtinShellGuard(settings ports.SettingsData, ev domain.ExpandEvent, payload map[string]any, stdin []byte) (domain.ExpandOutcome, bool) {
	if ev != domain.ExpandPreTool || settings.ExpandDisabled || slices.Contains(settings.ExpandOff, string(domain.SupertoolRules)) {
		return domain.ExpandOutcome{}, false
	}
	in := normalizeExpandPayload(payload, stdin)
	if domain.ClassifyTool(in.tool) != domain.ToolShell {
		return domain.ExpandOutcome{}, false
	}
	v := domain.EvaluateRules(domain.RuleEventBash, domain.RuleInput{Command: in.command}, domain.BuiltinRules())
	if v.Action != domain.RuleBlock {
		return domain.ExpandOutcome{}, false
	}
	return domain.ExpandOutcome{Decision: domain.DecisionBlock, Reason: v.Reason, Rule: v.Rule}, true
}

// expandInput es el payload normalizado de los tres harnesses.
type expandInput struct {
	sessionID, source, tool, command, filePath, content, prompt, lastMessage string
	// stopHookActive: el harness ya está continuando por un bloqueo previo en
	// stop (Claude Code y Codex). Volver a bloquear crearía un bucle.
	stopHookActive bool
}

// normalizeExpandPayload acepta la forma de Claude Code y Codex
// (`tool_name`, `tool_input`) y la del plugin de OpenCode (`tool`, `args`).
// Un payload que no es JSON se toma como prompt.
func normalizeExpandPayload(payload map[string]any, raw []byte) expandInput {
	if payload == nil {
		if json.Valid(raw) {
			return expandInput{}
		}
		return expandInput{prompt: strings.TrimSpace(string(raw))}
	}
	str := func(m map[string]any, keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	in := expandInput{
		sessionID:   str(payload, "session_id", "sessionID"),
		source:      str(payload, "source"),
		tool:        str(payload, "tool_name", "tool"),
		prompt:      str(payload, "prompt"),
		lastMessage: str(payload, "last_assistant_message"),
	}
	in.stopHookActive, _ = payload["stop_hook_active"].(bool)
	toolInput, _ := payload["tool_input"].(map[string]any)
	if toolInput == nil {
		toolInput, _ = payload["args"].(map[string]any)
	}
	if toolInput != nil {
		in.command = str(toolInput, "command", "cmd")
		// NotebookEdit trae notebook_path y new_source, y MultiEdit, el
		// contenido en edits[].new_string: sin ellos, las reglas de archivo
		// no ven ni la ruta ni lo insertado (A-001 de la ronda 5).
		in.filePath = str(toolInput, "file_path", "filePath", "path", "notebook_path")
		in.content = str(toolInput, "content", "new_string", "newString", "new_source")
		if in.content == "" {
			in.content = multiEditContent(toolInput)
		}
	}
	return in
}

// multiEditContent une los new_string de edits[] (MultiEdit), uno por línea.
func multiEditContent(toolInput map[string]any) string {
	edits, _ := toolInput["edits"].([]any)
	parts := make([]string, 0, len(edits))
	for _, e := range edits {
		if m, ok := e.(map[string]any); ok {
			if v, ok := m["new_string"].(string); ok && v != "" {
				parts = append(parts, v)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// oversizedShellPayload dice si un payload truncado por expandStdinMax es
// de una herramienta de shell. Lee el JSON incompleto por tokens hasta dar
// con tool_name/tool en el primer nivel: el comando que no se pudo leer
// entero no se puede analizar, así que no debe pasar sin más (B-11). Si el
// corte llega antes de saber la herramienta (`tool_input` primero y
// `tool_name` después), no se puede clasificar y se trata como shell: el
// orden de las claves no decide la seguridad (C-004 del ACR acr_a635820b).
func oversizedShellPayload(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	depth, key := 0, ""
	for {
		tok, err := dec.Token()
		if err != nil {
			return true
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else if depth--; depth == 1 {
				key = ""
			}
			continue
		}
		if depth != 1 {
			continue
		}
		if key == "" {
			key, _ = tok.(string)
			continue
		}
		if v, ok := tok.(string); ok && (key == "tool_name" || key == "tool") {
			return domain.ClassifyTool(v) == domain.ToolShell
		}
		key = ""
	}
}
