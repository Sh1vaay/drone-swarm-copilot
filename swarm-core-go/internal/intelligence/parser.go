package intelligence

import (
	"strings"

	"swarm-core-go/internal/core"
)

// ParseCommandIntent implements a high-efficiency deterministic keyword-map parser.
// It extracts intents instantly as a fallback when Gemini is unreachable:
// - "sweep sector alpha" -> Action: SWEEP, Target: alpha
// - "hold fleet position" -> Action: HOLD
// - "return and land" -> Action: LAND
func ParseCommandIntent(raw string) core.ParsedIntent {
	lower := strings.ToLower(strings.TrimSpace(raw))

	if strings.Contains(lower, "sweep") {
		target := "unknown"
		parts := strings.Fields(lower)
		for i, p := range parts {
			if (p == "sector" || p == "grid") && i+1 < len(parts) {
				target = parts[i+1]
				break
			}
		}
		return core.ParsedIntent{Action: "SWEEP", Target: target}
	}

	if strings.Contains(lower, "return") || strings.Contains(lower, "land") {
		return core.ParsedIntent{Action: "LAND"}
	}

	return core.ParsedIntent{Action: "HOLD", Target: "idle-sector"}
}
