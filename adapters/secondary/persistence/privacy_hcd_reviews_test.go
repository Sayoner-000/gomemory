package persistence

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

func TestPrivacyMaintenance_HCDDefinitionReviews(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	rawPayload, err := json.Marshal(domain.HCDDefinitionReviewPayload{
		Summary: "Resumen con <private>SYNTH-SECRET-HCD057</private> secreto",
		Quote:   "Cita pública",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insertar fila cruda sin proteger en hcd_definition_reviews
	_, err = db.Exec(`INSERT INTO hcd_definition_reviews (
		project, operation_id, unit_id, revision, kind, action, client, created_at, payload
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-proj", "op-priv-1", "HCD-001", "rev1", "socratic", "complete", "terminal", "2026-11-09T00:00:00Z", string(rawPayload),
	)
	if err != nil {
		t.Fatalf("insert raw review: %v", err)
	}

	store := NewPrivacyMaintenanceStore(db)

	var candidates []domain.SanitizeCandidate
	err = store.WithTransaction(ctx, "test-proj", func(tx ports.PrivacyTransaction) error {
		var err error
		candidates, err = tx.Candidates(ctx, "test-proj")
		if err != nil {
			return err
		}
		for _, c := range candidates {
			if c.Category == domain.SanitizeHCDDefinitionReview {
				safeText, err := sanitizeHCDDefinitionReviewPayload(c.Text)
				if err != nil {
					return err
				}
				applied, err := tx.Apply(ctx, c, safeText)
				if err != nil {
					return err
				}
				if !applied {
					t.Errorf("candidate %v not applied", c)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}

	// Verificar que hubo al menos 1 candidato de HCDDefinitionReview
	var found bool
	for _, c := range candidates {
		if c.Category == domain.SanitizeHCDDefinitionReview {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected SanitizeHCDDefinitionReview candidate found")
	}

	// Comprobar que en disco el secreto desapareció
	var savedPayload string
	err = db.QueryRow(`SELECT payload FROM hcd_definition_reviews WHERE project = ? AND operation_id = ?`, "test-proj", "op-priv-1").Scan(&savedPayload)
	if err != nil {
		t.Fatalf("query updated payload: %v", err)
	}
	if strings.Contains(savedPayload, "SYNTH-SECRET-HCD057") {
		t.Fatalf("expected secret redacted, got: %s", savedPayload)
	}

	// Compactar
	if err := store.Compact(ctx); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}
}
