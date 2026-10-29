package domain

import "fmt"

// Pointer values distinguish absent settings (use defaults) from invalid zero values.
type HCDSettings struct {
	HeartbeatTimeoutMS *int `json:"hcd_heartbeat_timeout_ms,omitempty"`
	GrantTimeoutHours  *int `json:"hcd_grant_timeout_hours,omitempty"`
	InactiveDays       *int `json:"hcd_inactive_days,omitempty"`
}

type ErrInvalidHCDSettings struct {
	Key   string
	Value int
}

func (e *ErrInvalidHCDSettings) Error() string {
	return fmt.Sprintf("invalid HCD setting %s: %d", e.Key, e.Value)
}

func ValidateHCDSettings(s HCDSettings) (HCDSettings, error) {
	for _, item := range []struct {
		key      string
		target   **int
		def, min int
	}{{"hcd_heartbeat_timeout_ms", &s.HeartbeatTimeoutMS, 200, 50}, {"hcd_grant_timeout_hours", &s.GrantTimeoutHours, 24, 1}, {"hcd_inactive_days", &s.InactiveDays, 7, 1}} {
		if *item.target == nil {
			v := item.def
			*item.target = &v
			continue
		}
		if **item.target < item.min {
			return s, &ErrInvalidHCDSettings{Key: item.key, Value: **item.target}
		}
	}
	return s, nil
}
