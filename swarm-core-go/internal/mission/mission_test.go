package mission

import (
	"testing"
	
	"swarm-core-go/internal/formation"
)

func TestCalculateProportionalAllocation(t *testing.T) {
	dronesBatteries := map[string]int32{
		"drone-A": 80,
		"drone-B": 20, // Sum = 100
	}
	totalWidth := 100.0

	widthA := formation.CalculateProportionalWidth("drone-A", dronesBatteries, totalWidth)
	widthB := formation.CalculateProportionalWidth("drone-B", dronesBatteries, totalWidth)

	if widthA != 80.0 {
		t.Errorf("Expected widthA to be 80.0, got %f", widthA)
	}
	if widthB != 20.0 {
		t.Errorf("Expected widthB to be 20.0, got %f", widthB)
	}
}

func TestValidateStateTransition(t *testing.T) {
	valid, _ := ValidateStateTransition("Idle", "Preflight")
	if !valid {
		t.Error("Expected Idle -> Preflight transition to be valid")
	}

	valid, _ = ValidateStateTransition("Executing", "Idle")
	if valid {
		t.Error("Expected Executing -> Idle transition to be rejected")
	}

	valid, _ = ValidateStateTransition("Executing", "Suspended")
	if !valid {
		t.Error("Expected Executing -> Suspended transition to be valid")
	}

	valid, _ = ValidateStateTransition("Suspended", "Landed")
	if !valid {
		t.Error("Expected Suspended -> Landed transition to be valid")
	}
}
