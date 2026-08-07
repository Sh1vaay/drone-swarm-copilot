package intelligence

import (
	"context"
	"strings"
	"testing"
	
	"swarm-core-go/internal/core"
)

func TestSanitizeOperatorInput(t *testing.T) {
	badInput := "<script>alert('hack')</script> sweep alpha!"
	cleaned := SanitizeOperatorInput(badInput)
	if !strings.Contains(cleaned, "sweep alpha") {
		t.Errorf("expected sweep command preserved, got %q", cleaned)
	}
	if strings.Contains(cleaned, "<script>") {
		t.Errorf("HTML must be stripped, got %q", cleaned)
	}

	badInput2 := "hold position \n\r\t"
	cleaned2 := SanitizeOperatorInput(badInput2)
	if cleaned2 != "hold position" {
		t.Errorf("control chars stripped: expected %q; got %q", "hold position", cleaned2)
	}

	long := strings.Repeat("a", 600)
	if len([]rune(SanitizeOperatorInput(long))) > maxOperatorInputRunes {
		t.Errorf("input must be capped at %d runes", maxOperatorInputRunes)
	}
}

func TestGeminiFailsafeFallback(t *testing.T) {
	intent := ParseIntentWithGemini(context.Background(), "sweep sector alpha", "")
	if intent.Action != "HOLD" {
		t.Errorf("Gemini no-key fallback should return HOLD, got %+v", intent)
	}
}

func TestHasFullPositionSet(t *testing.T) {
	if HasFullPositionSet(core.ParsedIntent{Positions: make([]core.DronePosition, 16)}, 16) != true {
		t.Error("expected full position set")
	}
	if HasFullPositionSet(core.ParsedIntent{Positions: make([]core.DronePosition, 10)}, 16) != false {
		t.Error("partial positions must not qualify")
	}
}
