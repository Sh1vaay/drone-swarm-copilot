package intelligence

import "testing"

func TestParseCommandIntent_Deterministic(t *testing.T) {
	cases := []struct {
		input  string
		action string
		target string
	}{
		{"sweep sector alpha", "SWEEP", "alpha"},
		{"sweep grid beta", "SWEEP", "beta"},
		{"hold fleet position", "HOLD", "idle-sector"},
		{"return and land", "LAND", ""},
		{"do some random flip", "HOLD", "idle-sector"},
		{"", "HOLD", "idle-sector"},
	}

	for _, tc := range cases {
		intent := ParseCommandIntent(tc.input)
		if intent.Action != tc.action {
			t.Errorf("For %q expected action %q, got %q", tc.input, tc.action, intent.Action)
		}
		if tc.target != "" && intent.Target != tc.target {
			t.Errorf("For %q expected target %q, got %q", tc.input, tc.target, intent.Target)
		}
	}
}
