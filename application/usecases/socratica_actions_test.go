package usecases

import (
	"errors"
	"slices"
	"testing"
	"time"

	"mem/domain"
)

// socraticaActionStates recorre todas las combinaciones de modo, fase y
// banderas que una superficie local puede mostrar.
func socraticaActionStates() []domain.RefinementFlow {
	var out []domain.RefinementFlow
	phases := []domain.SocraticaPhase{
		domain.PhaseCaptured, domain.PhaseQuestions, domain.PhaseRewriting, domain.PhaseReview,
		domain.PhaseApproved, domain.PhaseFailed, domain.PhaseDispatched, domain.PhaseCancelled,
	}
	for _, mode := range []domain.SocraticaMode{domain.ModeRefine, domain.ModeScrutiny, domain.ModeSpecify} {
		for _, phase := range phases {
			for flags := 0; flags < 16; flags++ {
				f := domain.RefinementFlow{
					ID: "f", Mode: mode, Phase: phase, LastStablePhase: domain.PhaseQuestions,
					OriginalPrompt: "original", ProposedPrompt: "propuesta", ApprovedPrompt: "aprobado",
					DispatchID: "d1", LastError: domain.ErrGenerationTimeout.Error(),
					InFlight: flags&1 != 0, Uncertain: flags&2 != 0, PrivateStateUnavailable: flags&4 != 0,
				}
				if flags&8 != 0 {
					f.LastError = "credenciales inválidas"
				}
				// Solo estados alcanzables: un envío incierto es una aprobación con
				// reclamación y sin lock, y solo specify guarda contenido efímero.
				if f.Uncertain && phase != domain.PhaseApproved || f.PrivateStateUnavailable && mode != domain.ModeSpecify {
					continue
				}
				if f.Uncertain {
					f.DispatchClaimOwner = "otro"
				}
				out = append(out, f)
			}
		}
	}
	return out
}

// domainEvent traduce las acciones que son eventos del dominio.
var domainEvent = map[SocraticaAction]domain.Event{
	ActionCancel:         {Kind: "cancel"},
	ActionEdit:           {Kind: "edit", Proposed: "editada"},
	ActionMarkDispatched: {Kind: "mark_dispatched"},
	ActionResend:         {Kind: "force_resend"},
	ActionRetry:          {Kind: "retry"},
	ActionSendOriginal:   {Kind: "send_original"},
}

// TestLocalActions_ElDominioAceptaTodoLoOfrecido: ninguna superficie puede
// ofrecer una acción que el dominio rechazaría en ese estado.
func TestLocalActions_ElDominioAceptaTodoLoOfrecido(t *testing.T) {
	u := &SocraticaFlow{}
	now := time.Unix(0, 0)
	for _, f := range socraticaActionStates() {
		for _, canAssist := range []bool{true, false} {
			offered := u.LocalActions(f, canAssist)
			for _, action := range offered {
				if ev, ok := domainEvent[action]; ok {
					ev.DispatchID = f.DispatchID
					if _, _, err := domain.Reduce(f, ev, now, func() string { return "nuevo" }); err != nil {
						t.Errorf("%s/%s %+v ofrece %s y el dominio lo rechaza: %v", f.Mode, f.Phase, flagsOf(f), action, err)
					}
				}
				switch action {
				case ActionExecute, ActionCopy:
					if f.Phase != domain.PhaseReview && f.Phase != domain.PhaseApproved {
						t.Errorf("%s/%s ofrece %s fuera de la revisión", f.Mode, f.Phase, action)
					}
				case ActionAssist, ActionChangeHarness:
					if f.Phase != domain.PhaseFailed {
						t.Errorf("%s/%s ofrece %s fuera de un fallo", f.Mode, f.Phase, action)
					}
				}
			}
			if f.InFlight && len(offered) != 0 {
				t.Errorf("un envío en curso no admite acciones: %v", offered)
			}
			if f.Mode == domain.ModeSpecify && len(offered) > 0 && !slices.Equal(offered, []SocraticaAction{ActionCancel}) {
				t.Errorf("specify solo puede cancelarse localmente: %v", offered)
			}
			if !canAssist && slices.Contains(offered, ActionAssist) {
				t.Errorf("sin sesión asistida no se ofrece continuar asistido")
			}
		}
	}
}

// TestLocalActions_CancelarDondeElDominioLoPermite: es el caso que la TUI
// anunciaba y luego rechazaba, una reserva enviada sin su contenido privado.
func TestLocalActions_CancelarDondeElDominioLoPermite(t *testing.T) {
	u := &SocraticaFlow{}
	now := time.Unix(0, 0)
	for _, f := range socraticaActionStates() {
		if f.InFlight {
			continue
		}
		_, _, err := domain.Reduce(f, domain.Event{Kind: "cancel"}, now, func() string { return "nuevo" })
		domainAllows := err == nil || !errors.Is(err, domain.ErrInvalidTransition) && !errors.Is(err, domain.ErrPrivateStateUnavailable)
		if f.Mode == domain.ModeSpecify && domainAllows != slices.Contains(u.LocalActions(f, true), ActionCancel) {
			t.Errorf("specify %s %+v: el dominio permite cancelar=%v, se ofrece=%v", f.Phase, flagsOf(f), domainAllows, !domainAllows)
		}
	}
}

func TestLocalActions_ReintentarSoloSiElFalloEsReintentable(t *testing.T) {
	u := &SocraticaFlow{}
	f := domain.RefinementFlow{Mode: domain.ModeRefine, Phase: domain.PhaseFailed, LastError: domain.ErrGenerationTimeout.Error()}
	if !slices.Contains(u.LocalActions(f, true), ActionRetry) {
		t.Error("un timeout debe poder reintentarse")
	}
	f.LastError = "credenciales inválidas"
	if slices.Contains(u.LocalActions(f, true), ActionRetry) {
		t.Error("un fallo no reintentable no ofrece reintentar")
	}
}

func flagsOf(f domain.RefinementFlow) map[string]bool {
	return map[string]bool{"in_flight": f.InFlight, "uncertain": f.Uncertain, "private_unavailable": f.PrivateStateUnavailable}
}
