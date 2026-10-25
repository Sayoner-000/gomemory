package usecases

import "mem/domain"

// SocraticaAction is something a person can do with a flow from a local
// surface (CLI or TUI). Which actions a state admits is decided only here: the
// surfaces map each one to their own key, label and order. Before, the TUI
// label, the TUI key handler and the CLI menus each kept their own copy and had
// drifted apart (finding H-03 of the 043 assessment).
type SocraticaAction string

const (
	ActionExecute        SocraticaAction = "execute"
	ActionEdit           SocraticaAction = "edit"
	ActionCopy           SocraticaAction = "copy"
	ActionAssist         SocraticaAction = "assist"
	ActionChangeHarness  SocraticaAction = "change_harness"
	ActionSendOriginal   SocraticaAction = "send_original"
	ActionMarkDispatched SocraticaAction = "mark_dispatched"
	ActionResend         SocraticaAction = "force_resend"
	ActionCancel         SocraticaAction = "cancel"
	ActionRetry          SocraticaAction = "retry"
)

// LocalActions lists what a person may do with f from a local surface.
// canAssist says whether the surface can open an assisted session (the CLI
// with --print cannot).
func (u *SocraticaFlow) LocalActions(f domain.RefinementFlow, canAssist bool) []SocraticaAction {
	switch {
	case f.InFlight:
		return nil
	case f.Mode == domain.ModeSpecify:
		// Preparation, review and delivery belong to the agent conversation:
		// locally a specify flow can only be cancelled, and only where the
		// domain allows it (a dispatched reservation without its payload too).
		if !f.Phase.Terminal() || (f.PrivateStateUnavailable && f.Phase == domain.PhaseDispatched) {
			return []SocraticaAction{ActionCancel}
		}
		return nil
	case f.Uncertain:
		return []SocraticaAction{ActionMarkDispatched, ActionResend, ActionCancel}
	}
	switch f.Phase {
	case domain.PhaseReview:
		return []SocraticaAction{ActionExecute, ActionEdit, ActionCopy, ActionCancel}
	case domain.PhaseApproved:
		// Approved but never run: the domain only edits in review, so editing
		// needs a resume first (as `pending --resume` does).
		return []SocraticaAction{ActionExecute, ActionCopy, ActionCancel}
	case domain.PhaseFailed:
		var out []SocraticaAction
		if canAssist {
			out = append(out, ActionAssist)
		}
		out = append(out, ActionChangeHarness, ActionSendOriginal, ActionCancel)
		if u.ExplainFailure(f.LastError).Retryable {
			out = append(out, ActionRetry)
		}
		return out
	case domain.PhaseDispatched:
		return []SocraticaAction{ActionResend}
	}
	return nil
}
