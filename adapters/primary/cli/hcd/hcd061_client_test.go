package hcd

import "testing"

// TestHCD061ClientePi: Pi se identifica como cliente propio para que sus
// aprobaciones conserven la procedencia (HCD-061).
func TestHCD061ClientePi(t *testing.T) {
	p, err := parse([]string{"start", "--client", "pi"})
	if err != nil || p.client != "pi" {
		t.Fatalf("parse(start --client pi) = %+v %v", p, err)
	}
}
