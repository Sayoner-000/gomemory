package domain

import "testing"

func TestValidQuote(t *testing.T) {
	for _, quote := range []string{"a1b2c3d4e5f6", "Apruebo", "apruebo", "aprobar", "sí, aprobar", "si aprobar", "sí apruebo"} {
		if !ValidQuote(quote, "a1b2c3d4e5f6") {
			t.Errorf("valid quote rejected: %q", quote)
		}
	}
	for _, quote := range []string{"", "A", "recommended", "no estoy seguro", "no apruebo", "rechazo", "quizá aprobar"} {
		if ValidQuote(quote, "a1b2c3d4e5f6") {
			t.Errorf("invalid quote accepted: %q", quote)
		}
	}
}

func TestValidQuoteRejectsProtectedTextWithoutApproval(t *testing.T) {
	if ValidQuote("<private>no</private>", "a1b2c3d4e5f6") {
		t.Fatal("protected text cannot count as approval")
	}
}
