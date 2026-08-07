package formation

import (
	"math"
	"testing"

	"swarm-core-go/internal/core"
)

func TestElectSwarmLeader(t *testing.T) {
	drones := []core.MockDroneState{
		{DroneID: "drone-A", Battery: 45, Alt: 1.0},
		{DroneID: "drone-B", Battery: 88, Alt: 1.2},
		{DroneID: "drone-C", Battery: 12, Alt: 1.5}, // Low battery
	}

	// Drone-B has the highest battery, should be elected leader
	leaderID := ElectSwarmLeader(drones)
	if leaderID != "drone-B" {
		t.Errorf("Expected leader drone-B, got %s", leaderID)
	}
}

func TestLeaderHandoverTrigger(t *testing.T) {
	// Active leader has battery 29% (below 30% threshold)
	shouldHandover := EvaluateHandoverNecessity("drone-A", 29, "drone-B", 85)
	if !shouldHandover {
		t.Error("Expected leader handover to be required when battery < 30%")
	}

	// Active leader has battery 75%, candidate has 78% (flapping filter prevents handover, difference < 5%)
	shouldHandover = EvaluateHandoverNecessity("drone-A", 75, "drone-B", 78)
	if shouldHandover {
		t.Error("Expected leader handover to be blocked by role flapping filter (difference < 5%)")
	}

	// Active leader has battery 70%, candidate has 80% (difference >= 5%, should handover)
	shouldHandover = EvaluateHandoverNecessity("drone-A", 70, "drone-B", 80)
	if !shouldHandover {
		t.Error("Expected leader handover to trigger since candidate battery is >= 5% higher")
	}
}

func TestExponentialDecayDamping(t *testing.T) {
	currentX := 1.0
	targetX := 5.0
	alpha := 0.8

	// Step 1: Smooth reference transfer (damping updates)
	smoothedX := SmoothCoordinate(currentX, targetX, alpha)
	expectedX := 1.0*0.8 + 5.0*0.2 // 1.8
	if math.Abs(smoothedX-expectedX) > 1e-9 {
		t.Errorf("SmoothCoordinate() = %v; want %v", smoothedX, expectedX)
	}
}
