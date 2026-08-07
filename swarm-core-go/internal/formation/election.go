package formation

import (
	"log"

	"swarm-core-go/internal/core"
)

// ElectSwarmLeader finds the best drone based on multiple weighted criteria for maximum fault tolerance
func ElectSwarmLeader(drones []core.MockDroneState) string {
	if len(drones) == 0 {
		return ""
	}

	bestID := ""
	maxScore := -1000.0 // start with a very low baseline

	for _, d := range drones {
		// Only consider drones above the min altimeter threshold (20cm)
		if d.Alt >= 0.20 {
			// Normalize battery to 0.0 - 1.0
			normBattery := float64(d.Battery) / 100.0

			// Scoring Weights:
			// 40% Battery
			// 30% Trust Score
			// 20% Communication Quality
			// 10% Proximity (Penalized by distance from center)
			score := (0.40 * normBattery) +
				(0.30 * d.TrustScore) +
				(0.20 * d.CommQuality) -
				(0.01 * d.DistanceToCenter) // Since dist^2 is passed, small penalty

			if score > maxScore {
				maxScore = score
				bestID = d.DroneID
			}
		}
	}

	return bestID
}

// EvaluateHandoverNecessity checks if active leader roles must transfer to a candidate
func EvaluateHandoverNecessity(currentLeaderID string, currentBattery int32, candidateID string, candidateBattery int32) bool {
	if currentLeaderID == "" {
		return true // No active leader, handover required immediately
	}

	// 1. Hard battery depletion threshold (BR-02)
	if currentBattery < 30 {
		log.Printf("Leader %s battery depleted below 30%% threshold (%d%%). Re-election triggered.", currentLeaderID, currentBattery)
		return true
	}

	// 2. Flapping filter (Candidate must exceed current leader battery by at least 5% threshold to change)
	batteryDifference := candidateBattery - currentBattery
	if batteryDifference >= 5 {
		log.Printf("Candidate %s battery (%d%%) is >= 5%% higher than leader %s (%d%%). Triggering handover.", candidateID, candidateBattery, currentLeaderID, currentBattery)
		return true
	}

	return false
}
