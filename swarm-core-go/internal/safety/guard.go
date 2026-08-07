package safety

import (
	"math"

	"swarm-core-go/internal/core"
)

// CalculateSeparationThreshold computes the dynamic safety threshold based on velocity (BR-02).
func CalculateSeparationThreshold(velocity float64) float64 {
	baseThreshold := 0.5
	if velocity <= 2.0 {
		return baseThreshold
	}
	return baseThreshold + 0.25*(velocity-2.0)
}

// VerifyProximityBreach calculates distance and computes a spatial repulsion escape vector if breached.
func VerifyProximityBreach(x1, y1, z1, x2, y2, z2, velocity float64) (bool, core.Vector3D) {
	dx := x1 - x2
	dy := y1 - y2
	dz := z1 - z2
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)

	threshold := CalculateSeparationThreshold(velocity)
	if distance >= threshold {
		return false, core.Vector3D{}
	}

	if distance == 0 {
		return true, core.Vector3D{X: 1.0, Y: 0.0, Z: 0.0}
	}

	return true, core.Vector3D{
		X: dx / distance,
		Y: dy / distance,
		Z: dz / distance,
	}
}

// CheckFlightBoundaries verifies altitude limits using DefaultEnvelope.
// Uses MinGroundZ as the lower bound so drones at ground-level spawn altitude
// (0.05m) are not incorrectly flagged. MinAltitudeM (0.20m) is the sustained
// flight target, not the absolute coordinate floor.
func CheckFlightBoundaries(alt float64) (bool, string) {
	env := core.DefaultEnvelope
	if alt < env.MinGroundZ {
		return false, "Altitude boundary breach: below minimum safety limit (20cm)"
	}
	if alt > env.MaxAltitudeM {
		return false, "Altitude boundary breach: above maximum safe volume"
	}
	return true, ""
}
