package formation

import (
	"testing"

	"swarm-core-go/internal/core"
)

func TestElectSwarmLeader_WeightedScore(t *testing.T) {
	drones := []core.MockDroneState{
		{
			DroneID:          "Drone-A",
			Battery:          100,
			Alt:              2.0,
			CommQuality:      0.1,
			TrustScore:       0.1,
			DistanceToCenter: 50.0,
		},
		{
			DroneID:          "Drone-B",
			Battery:          80,
			Alt:              2.0,
			CommQuality:      1.0,
			TrustScore:       1.0,
			DistanceToCenter: 0.0,
		},
	}

	winner := ElectSwarmLeader(drones)
	if winner != "Drone-B" {
		t.Errorf("Expected Drone-B to win due to better overall weighted score, got %s", winner)
	}
}

func TestElectSwarmLeader_AltitudeThreshold(t *testing.T) {
	drones := []core.MockDroneState{
		{
			DroneID:          "Drone-A",
			Battery:          100,
			Alt:              0.05, // Below threshold
			CommQuality:      1.0,
			TrustScore:       1.0,
			DistanceToCenter: 0.0,
		},
		{
			DroneID:          "Drone-B",
			Battery:          20,
			Alt:              2.0, // Above threshold
			CommQuality:      0.5,
			TrustScore:       0.5,
			DistanceToCenter: 10.0,
		},
	}

	winner := ElectSwarmLeader(drones)
	if winner != "Drone-B" {
		t.Errorf("Expected Drone-B to win because Drone-A is below altitude threshold, got %s", winner)
	}
}
