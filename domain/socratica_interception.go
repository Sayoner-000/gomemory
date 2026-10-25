package domain

import (
	"strings"
	"time"
	"unicode"
)

// socraticaCommands are the explicit invocations recognised in any agent:
// slash commands (Claude Code, OpenCode) and Codex's $skill form. The Codex
// form of mode=specify ("$socratica prepara para speckit-specify: …") is a
// $socratica invocation too.
var socraticaCommands = []string{"/socratica", "/socratic-q", "/socratica-hu", "$socratica"}

// accentFolder lets the natural request match with or without accents.
var accentFolder = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u")

// isNaturalSpecifyRequest recognises the explicit natural-language entry of
// mode=specify: a message that starts by asking Socratica to prepare something
// and names Spec Kit. It is deliberately narrow: a sentence that merely
// mentions Socratica or Spec Kit elsewhere is not an invocation.
func isNaturalSpecifyRequest(t string) bool {
	rest, ok := strings.CutPrefix(accentFolder.Replace(strings.ToLower(t)), "usa socratica para ")
	return ok && (strings.Contains(rest, "spec kit") || strings.Contains(rest, "speckit"))
}

// IsSocraticaInvocation reports a prompt that starts with an explicit Socratica
// command, alone or followed by whitespace, or that is the natural request for
// mode=specify.
func IsSocraticaInvocation(text string) bool {
	t := strings.TrimSpace(text)
	for _, cmd := range socraticaCommands {
		if rest, ok := strings.CutPrefix(t, cmd); ok && (rest == "" || strings.TrimLeftFunc(rest, unicode.IsSpace) != rest) {
			return true
		}
	}
	return isNaturalSpecifyRequest(t)
}

// OriginSocraticaApproved marks a prompt that is the approved text of a
// Socratica dispatch: it must never be captured again.
const OriginSocraticaApproved = "socratica-approved"

// SocraticaDirective is injected when a prompt is captured: the original stays
// in the conversation and the agent enters the skill flow instead of acting.
const SocraticaDirective = "Socratica (intercepción automática): antes de hacer nada, aplica la habilidad `socratica` en modo refine al prompt de este turno. No ejecutes la tarea: sigue el flujo y espera aprobación explícita. Para enviar un mensaje sin refinar, la persona lo empieza por `>`."

// SocraticaForwardDirective asks the agent to drop the escape prefix where the
// host cannot rewrite the prompt itself (Claude Code, Codex).
const SocraticaForwardDirective = "Socratica: la persona empezó el mensaje con `>` para enviarlo sin refinar. Trátalo sin ese primer `>` y sin aplicar Socratica."

// InterceptionInput are the signals a hook gathers for one prompt.
type InterceptionInput struct {
	Text                  string
	Enabled               bool
	Attachments, Busy     bool
	PendingPhase          SocraticaPhase
	PendingUpdatedAt, Now time.Time
	Origin                string
}

// InterceptionDecision: Forward is the text without its escape ">" (exactly
// one is removed) and is set only for an escaped prompt.
type InterceptionDecision struct {
	Capture   bool
	Forward   string
	Forwarded bool
	Directive string
}

// DecideInterception is the whole capture rule (research R5). Interception
// never rewrites or blocks a prompt: it only adds a directive.
func DecideInterception(in InterceptionInput) InterceptionDecision {
	text := strings.TrimSpace(in.Text)
	if !in.Enabled || text == "" || in.Origin == OriginSocraticaApproved {
		return InterceptionDecision{}
	}
	if rest, ok := strings.CutPrefix(strings.TrimLeft(in.Text, " \t"), ">"); ok {
		return InterceptionDecision{Forward: rest, Forwarded: true, Directive: SocraticaForwardDirective}
	}
	pending := (in.PendingPhase == PhaseQuestions || in.PendingPhase == PhaseReview) && in.Now.Sub(in.PendingUpdatedAt) <= ContinuationWindow
	switch {
	case strings.HasPrefix(text, "/"), IsSocraticaInvocation(text), in.Attachments, in.Busy, pending,
		WordCount(text) <= LowInfoMaxWords:
		return InterceptionDecision{}
	}
	return InterceptionDecision{Capture: true, Directive: SocraticaDirective}
}
