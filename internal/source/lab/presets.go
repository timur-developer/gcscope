package lab

import (
	"sort"
	"strings"
)

const (
	PresetAlloc = "alloc"
	PresetChurn = "churn"
	PresetIdle  = "idle"
	PresetSpike = "spike"
)

var presets = map[string]struct{}{
	PresetAlloc: {},
	PresetChurn: {},
	PresetIdle:  {},
	PresetSpike: {},
}

func IsValidPreset(name string) bool {
	_, ok := presets[name]
	return ok
}

func AvailablePresets() []string {
	out := make([]string, 0, len(presets))
	for p := range presets {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func AvailablePresetsString() string {
	return strings.Join(AvailablePresets(), ", ")
}
