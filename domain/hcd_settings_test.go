package domain

import "testing"

func TestValidateHCDSettingsDefaultsAndRanges(t *testing.T) {
	got, err := ValidateHCDSettings(HCDSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if *got.HeartbeatTimeoutMS != 200 || *got.GrantTimeoutHours != 24 || *got.InactiveDays != 7 {
		t.Fatalf("defaults = %+v", got)
	}
	valid := HCDSettings{HeartbeatTimeoutMS: intPtr(50), GrantTimeoutHours: intPtr(1), InactiveDays: intPtr(1)}
	if _, err := ValidateHCDSettings(valid); err != nil {
		t.Fatalf("minimums must be accepted: %v", err)
	}
	for _, settings := range []HCDSettings{{HeartbeatTimeoutMS: intPtr(49)}, {GrantTimeoutHours: intPtr(0)}, {InactiveDays: intPtr(0)}} {
		if _, err := ValidateHCDSettings(settings); err == nil {
			t.Errorf("invalid settings accepted: %+v", settings)
		}
	}
}

func intPtr(n int) *int { return &n }
