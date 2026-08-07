package safety_test

import (
	"math"
	"testing"

	"swarm-core-go/internal/core"
	"swarm-core-go/internal/formation"
	"swarm-core-go/internal/intelligence"
	"swarm-core-go/internal/mission"
	"swarm-core-go/internal/safety"
)

// TestSwarmMigrationPlatformGoals runs a complete virtual simulation validating all migration goals:
// 1. Concurrent multi-drone state management
// 2. Telemetry-driven leader election and battery failovers
// 3. Dynamic formation solver with exponential coord damping (alpha = 0.8)
// 4. Gemini NLP Command parsing and battery-proportional grid allocations
// 5. Strict safety boundary checks (separation repulsion and altimeter lock)
func TestSwarmMigrationPlatformGoals(t *testing.T) {
	// =========================================================================
	// GOAL 1: Natural Language Voice-Command parsing (Gemini Live)
	// =========================================================================
	t.Run("Gemini Live Natural Language Intent Parser", func(t *testing.T) {
		// ParseCommandIntent is now a safe-only HOLD fallback.
		// All real NLP is done by ParseIntentWithGemini (requires live API).
		// Verify the safe fallback always returns HOLD for any input.
		intent1 := intelligence.ParseCommandIntent("sweep sector alpha")
		if intent1.Action != "HOLD" {
			t.Errorf("Safe fallback must always be HOLD, got %+v", intent1)
		}

		intent2 := intelligence.ParseCommandIntent("hold fleet position")
		if intent2.Action != "HOLD" {
			t.Errorf("Safe fallback must always be HOLD, got %+v", intent2)
		}

		intent3 := intelligence.ParseCommandIntent("return and land")
		if intent3.Action != "HOLD" {
			t.Errorf("Safe fallback must always be HOLD, got %+v", intent3)
		}

		// Gibberish also returns HOLD
		intentFallback := intelligence.ParseCommandIntent("some completely unrecognized gibberish")
		if intentFallback.Action != "HOLD" {
			t.Errorf("NLP Fallback failed: expected HOLD; got %+v", intentFallback)
		}
	})

	// =========================================================================
	// GOAL 2: Cooperative Task Grid Allocation (Proportional Battery Split)
	// =========================================================================
	t.Run("Battery-Proportional Grid Slicer", func(t *testing.T) {
		activeBatteries := map[string]int32{
			"drone-sim-1": 70, // 70% battery
			"drone-sim-2": 30, // 30% battery (Sum = 100)
		}
		totalSearchWidth := 200.0 // 200 meters sector

		width1 := formation.CalculateProportionalWidth("drone-sim-1", activeBatteries, totalSearchWidth)
		width2 := formation.CalculateProportionalWidth("drone-sim-2", activeBatteries, totalSearchWidth)

		if width1 != 140.0 {
			t.Errorf("Proportional Allocation failed for Drone 1: expected 140m; got %f", width1)
		}
		if width2 != 60.0 {
			t.Errorf("Proportional Allocation failed for Drone 2: expected 60m; got %f", width2)
		}
	})

	// =========================================================================
	// GOAL 3: Strict Dynamic Mission State Machine
	// =========================================================================
	t.Run("Mission State Machine Transition Flow", func(t *testing.T) {
		currentState := "Idle"

		// Preflight is valid from Idle
		valid, err := mission.ValidateStateTransition(currentState, "Preflight")
		if !valid || err != nil {
			t.Errorf("Idle -> Preflight failed: %v", err)
		}

		// Transition directly from Executing to Idle must be strictly blocked (requires landing first)
		valid, err = mission.ValidateStateTransition("Executing", "Idle")
		if valid || err == nil {
			t.Error("Safety violation: Direct transition from Executing to Idle must be blocked")
		}

		// Executing -> Landed: Valid
		valid, err = mission.ValidateStateTransition("Executing", "Landed")
		if !valid || err != nil {
			t.Errorf("Executing -> Landed failed: %v", err)
		}
	})

	// =========================================================================
	// GOAL 4: Dynamic Leader Election & Battery Failover
	// =========================================================================
	t.Run("Telemetry-Driven Leader Election & Failover Handover", func(t *testing.T) {
		// Drones active in swarm
		drones := []core.MockDroneState{
			{DroneID: "drone-A", Battery: 35, Alt: 1.2},
			{DroneID: "drone-B", Battery: 85, Alt: 1.5},
			{DroneID: "drone-C", Battery: 12, Alt: 1.0},
		}

		// Startup leader election: Drone-B has the highest battery
		leaderID := formation.ElectSwarmLeader(drones)
		if leaderID != "drone-B" {
			t.Errorf("Expected startup elected leader to be drone-B; got %s", leaderID)
		}

		// Simulate Drone-B (leader) battery depletion down to 29% (< 30% reelection threshold)
		droneBBattery := int32(29)
		droneABattery := int32(80) // Drone-A is now the best candidate

		shouldHandover := formation.EvaluateHandoverNecessity("drone-B", droneBBattery, "drone-A", droneABattery)
		if !shouldHandover {
			t.Error("Reelection failed to trigger when leader's battery dropped below 30%")
		}
	})

	// =========================================================================
	// GOAL 5: Dynamic Formation Solver & Exponential Coordination Damping
	// =========================================================================
	t.Run("Spatial Offset Solver & Coordinate Smooth Damping", func(t *testing.T) {
		// Verify Circle coordinates computation
		circleOffsets := formation.ComputeFormationOffsets("circle", 3, 2.0)
		if len(circleOffsets) != 3 {
			t.Errorf("Circle formation offsets failed: expected 3 points, got %d", len(circleOffsets))
		}

		// Verify V-Shape coordinates computation (key is now v_shape)
		vOffsets := formation.ComputeFormationOffsets("v_shape", 3, 2.0)
		if len(vOffsets) != 3 {
			t.Errorf("V-shape formation offsets failed: expected 3 points, got %d", len(vOffsets))
		}
		// Leader at center: (0,0)
		if vOffsets[0].X != 0 || vOffsets[0].Y != 0 {
			t.Errorf("V-shape leader offset failed: expected (0,0); got (%f,%f)", vOffsets[0].X, vOffsets[0].Y)
		}

		// Validate asymptotic coordinates reference tracking with alpha = 0.8
		currentCoord := 1.0
		targetReferenceCoord := 10.0
		alpha := 0.8

		// Step 1: Smooth reference transfer (alpha damp)
		smoothedCoord := formation.SmoothCoordinate(currentCoord, targetReferenceCoord, alpha)
		expectedStep1 := 1.0*0.8 + 10.0*0.2 // 2.8
		if math.Abs(smoothedCoord-expectedStep1) > 1e-9 {
			t.Errorf("Decay smoothing failed: Step 1 expected %f, got %f", expectedStep1, smoothedCoord)
		}

		// Step 2: Continuation towards target reference
		smoothedCoord = formation.SmoothCoordinate(smoothedCoord, targetReferenceCoord, alpha)
		expectedStep2 := 2.8*0.8 + 10.0*0.2 // 4.24
		if math.Abs(smoothedCoord-expectedStep2) > 1e-9 {
			t.Errorf("Decay smoothing failed: Step 2 expected %f, got %f", expectedStep2, smoothedCoord)
		}
	})

	// =========================================================================
	// GOAL 6: Rigid Safety Guard Rails (Collision Repulsion & Altimeter Lock)
	// =========================================================================
	t.Run("Collision Avoidance Spatial Escape & Altimeter Boundaries Lock", func(t *testing.T) {
		// 1. Separation Threshold scaling formula under 2 m/s velocity
		thresholdLowSpeed := safety.CalculateSeparationThreshold(1.5)
		if thresholdLowSpeed != 0.5 { // base bubble: 50cm
			t.Errorf("Separation threshold error at 1.5 m/s: expected 0.5m, got %f", thresholdLowSpeed)
		}

		// Separation Threshold scaling formula exceeding 2 m/s velocity: 50cm + 25cm per m/s exceeding 2.0
		thresholdHighSpeed := safety.CalculateSeparationThreshold(4.0)
		expectedThreshold := 0.5 + 0.25*(4.0-2.0) // 1.0 meter safety bubble
		if thresholdHighSpeed != expectedThreshold {
			t.Errorf("Dynamic safety bubble scaling failed at 4.0 m/s: expected %f, got %f", expectedThreshold, thresholdHighSpeed)
		}

		// 2. Spatial Avoidance Proximity Breach trigger and Escape repulsion vector
		// Place Drone A at (0.0, 0.0, 1.0) and Drone B at (0.0, 0.3, 1.0) -> distance = 0.3m (less than 0.5m threshold)
		breached, escape := safety.VerifyProximityBreach(0.0, 0.0, 1.0, 0.0, 0.3, 1.0, 1.0)
		if !breached {
			t.Error("Avoidance Guard failed to trigger proximity breach for 30cm separation")
		}
		// Escape vector must point directly away on the Y-axis (dx=0, dy=-0.3, dz=0 -> normalized: Y=-1.0)
		if escape.Y >= 0 || math.Abs(escape.Y+1.0) > 1e-9 {
			t.Errorf("Escape repulsion vector calculation failed: expected Y = -1.0; got %+v", escape)
		}

		// 3. Altimeter minimum lock
		safe, err := safety.CheckFlightBoundaries(0.15) // below 20cm
		if safe || err == "" {
			t.Error("Altimeter Lock failed: accepted unsafe altitude below 20cm")
		}

		safe, err = safety.CheckFlightBoundaries(1.5) // safe
		if !safe || err != "" {
			t.Errorf("Altimeter Lock error on safe coordinate: %s", err)
		}
	})
}
