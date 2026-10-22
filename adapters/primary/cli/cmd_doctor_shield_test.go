package cli

import (
	"encoding/json"
	"mem/domain"
	"strings"
	"testing"
)

func TestDoctorShieldJSONAndCodes(t *testing.T) {
	for _, tc := range []struct {
		severity string
		code     int
	}{{"critical", 2}, {"high", 1}, {"medium", 0}, {"", 0}} {
		r := domain.ShieldReport{Skipped: []string{"large.json"}}
		if tc.severity != "" {
			r.Findings = []domain.ShieldFinding{{Severity: tc.severity}}
		}
		if got := shieldExitCode(r); got != tc.code {
			t.Errorf("%s: %d", tc.severity, got)
		}
		raw, _ := json.Marshal(doctorReportJSON{Shield: &r})
		if !strings.Contains(string(raw), `"shield"`) || !strings.Contains(string(raw), `"skipped":["large.json"]`) {
			t.Fatal(string(raw))
		}
	}
	raw, _ := json.Marshal(doctorReportJSON{})
	if strings.Contains(string(raw), `"shield"`) {
		t.Fatal("changed doctor without --shield")
	}
}
