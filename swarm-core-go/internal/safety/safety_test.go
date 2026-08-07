package safety

import (
	"math"
	"testing"
)

func TestCalculateSeparationThreshold(t *testing.T) {
	tests := []struct {
		name     string
		velocity float64
		expected float64
	}{
		{"Low Speed", 1.5, 0.5},
		{"Threshold Speed", 2.0, 0.5},
		{"High Speed Moderate", 4.0, 1.0}, // 50cm + 25cm * (4.0 - 2.0) = 1.0m
		{"Extreme Speed", 6.0, 1.5},        // 50cm + 25cm * (6.0 - 2.0) = 1.5m
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSeparationThreshold(tt.velocity)
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("CalculateSeparationThreshold(%v) = %v; want %v", tt.velocity, got, tt.expected)
			}
		})
	}
}

func TestVerifyProximityBreach(t *testing.T) {
	// Drone A at (0, 0, 1.0) and Drone B at (0, 0, 1.2) - distance is 20cm
	// Speed is 4.0 m/s - separation threshold is 1.0m (100cm)
	// Breach should be triggered
	breach, escape := VerifyProximityBreach(0, 0, 1.0, 0, 0, 1.2, 4.0)
	if !breach {
		t.Error("Expected proximity breach to be triggered")
	}

	// Escape vector z-axis should point away from Drone B (since B is at 1.2 and A is at 1.0, A should translate down)
	if escape.Z >= 0 {
		t.Errorf("Expected escape z-vector to point downwards (< 0), got %v", escape.Z)
	}
}
