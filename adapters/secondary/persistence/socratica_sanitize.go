package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mem/domain"
	"strings"
)

type socraticaSensitiveColumn struct {
	table, column string
	isJSON        bool
}

func socraticaSanitizeColumn(cat domain.SanitizeCategory) (socraticaSensitiveColumn, bool) {
	prefix := string(cat)
	table := ""
	if strings.HasPrefix(prefix, "socratica_flow_") {
		table = "socratica_flows"
		prefix = strings.TrimPrefix(prefix, "socratica_flow_")
	} else if strings.HasPrefix(prefix, "socratica_history_") {
		table = "socratica_history"
		prefix = strings.TrimPrefix(prefix, "socratica_history_")
	} else {
		return socraticaSensitiveColumn{}, false
	}
	col := map[string]string{"original": "original_prompt", "proposed": "proposed_prompt", "approved": "approved_prompt", "questions": "questions_json", "answers": "answers_json", "error": "last_error", "scrutiny": "scrutiny_json", "additions": "additions_json", "preparation": "preparation_json"}[prefix]
	if col == "" {
		return socraticaSensitiveColumn{}, false
	}
	return socraticaSensitiveColumn{table, col, strings.HasSuffix(col, "_json")}, true
}
func socraticaSanitizeText(raw string, isJSON bool) (string, error) {
	if !isJSON {
		return domain.ProtectText(raw), nil
	}
	var tree any
	if err := json.Unmarshal([]byte(raw), &tree); err != nil {
		return "", fmt.Errorf("invalid socratica JSON: %w", domain.ErrValidation)
	}
	return protectSocraticaJSON(tree)
}
func readSocraticaCandidates(ctx context.Context, q queryer, project string, add func(domain.SanitizeCategory, string, string, string)) error {
	groups := []struct {
		prefix string
		names  []string
	}{{"socratica_flow_", []string{"original", "proposed", "approved", "questions", "answers", "error", "scrutiny", "preparation"}}, {"socratica_history_", []string{"original", "proposed", "approved", "additions", "scrutiny", "preparation"}}}
	for _, group := range groups {
		for _, name := range group.names {
			cat := domain.SanitizeCategory(group.prefix + name)
			col, _ := socraticaSanitizeColumn(cat)
			err := scanRows(ctx, q, "SELECT id,"+col.column+" FROM "+col.table+" WHERE project=?", project, func(row *sql.Rows) error {
				var id string
				var raw sql.NullString
				if err := row.Scan(&id, &raw); err != nil {
					return err
				}
				if !raw.Valid {
					return nil
				}
				safe, err := socraticaSanitizeText(raw.String, col.isJSON)
				if err != nil {
					return err
				}
				// Formatting alone is not a privacy residue. Compare decoded leaves for JSON.
				if col.isJSON {
					var tree any
					if err = json.Unmarshal([]byte(raw.String), &tree); err != nil {
						return err
					}
					canonical, e := json.Marshal(tree)
					if e != nil {
						return e
					}
					if safe != string(canonical) {
						add(cat, col.column, id, raw.String)
					}
				} else if safe != raw.String {
					add(cat, col.column, id, raw.String)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func applySocraticaCandidate(ctx context.Context, tx *sql.Tx, col socraticaSensitiveColumn, c domain.SanitizeCandidate) (bool, error) {
	if col.table == "socratica_flows" && col.column == "preparation_json" {
		return applySocraticaFlowPreparation(ctx, tx, c)
	}
	text, err := socraticaSanitizeText(c.Text, col.isJSON)
	if err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, "UPDATE "+col.table+" SET "+col.column+"=? WHERE id=? AND "+col.column+"=? AND project=?", text, c.RecordID, c.Text, c.Project)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// applySocraticaFlowPreparation protects the stored preparation of a flow. When
// the content changes, the review and the authorization that were built on the
// old content cannot survive: an unconsumed review or approval returns to
// preparation with a new revision, so nothing approved before the cleaning can
// be delivered afterwards. A consumed or claimed dispatch keeps only its marks,
// and a preparation of a newer version is protected without being reinterpreted.
func applySocraticaFlowPreparation(ctx context.Context, tx *sql.Tx, c domain.SanitizeCandidate) (bool, error) {
	safe, err := socraticaSanitizeText(c.Text, true)
	if err != nil {
		return false, err
	}
	var phase string
	var acked bool
	var claim sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT phase,dispatch_ack,dispatch_claim_owner FROM socratica_flows WHERE id=? AND project=? AND preparation_json=?", c.RecordID, c.Project, c.Text).Scan(&phase, &acked, &claim)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // changed meanwhile: the next pass sees the new value
	}
	if err != nil {
		return false, err
	}
	var prep domain.SpecifyPreparation
	if err = json.Unmarshal([]byte(safe), &prep); err != nil {
		return false, err
	}
	update := "UPDATE socratica_flows SET preparation_json=?"
	if prep.SchemaVersion <= domain.PreparationSchemaVersion && !acked && !claim.Valid {
		switch domain.SocraticaPhase(phase) {
		case domain.PhaseReview, domain.PhaseApproved:
			prep.Review, prep.Checks, prep.Stage = nil, nil, domain.StageIntent
			raw, e := json.Marshal(prep)
			if e != nil {
				return false, e
			}
			safe = string(raw)
			update += ",phase='captured',last_stable_phase='captured',proposed_prompt='',approved_prompt='',approved_hash='',dispatch_id=NULL,revision=revision+1"
		case domain.PhaseCaptured, domain.PhaseQuestions:
			// Checks over the old content would block the next validation.
			if domain.RewindStaleChecks(&prep) {
				raw, e := json.Marshal(prep)
				if e != nil {
					return false, e
				}
				safe = string(raw)
			}
			update += ",revision=revision+1"
		}
	}
	res, err := tx.ExecContext(ctx, update+" WHERE id=? AND project=? AND preparation_json=?", safe, c.RecordID, c.Project, c.Text)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
