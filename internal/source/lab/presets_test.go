package lab

import "testing"

func TestIsValidPreset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		valid bool
	}{
		{name: PresetAlloc, valid: true},
		{name: PresetChurn, valid: true},
		{name: PresetIdle, valid: true},
		{name: PresetSpike, valid: true},
		{name: "", valid: false},
		{name: "unknown", valid: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsValidPreset(tc.name); got != tc.valid {
				t.Fatalf("IsValidPreset(%q)=%v, want %v", tc.name, got, tc.valid)
			}
		})
	}
}

func TestAvailablePresetsString(t *testing.T) {
	t.Parallel()

	got := AvailablePresetsString()
	if got == "" {
		t.Fatalf("AvailablePresetsString() is empty")
	}
}
