package core

// DefaultEnvelope is the single source of truth for operational altitude limits (metres).
var DefaultEnvelope = FlightEnvelope{
	MinAltitudeM: 0.20,
	MaxAltitudeM: 12.0,
	MinGroundZ:   0.05,
	MaxCoordXY:   8.0,
}

// FlightEnvelope defines safe operational bounds for the swarm simulation.
type FlightEnvelope struct {
	MinAltitudeM float64
	MaxAltitudeM float64
	MinGroundZ   float64
	MaxCoordXY   float64
}

// ClampAltitude constrains altitude to the envelope.
func (e FlightEnvelope) ClampAltitude(z float64) float64 {
	return clamp(z, e.MinGroundZ, e.MaxAltitudeM)
}

// ClampCoord constrains horizontal offset to the envelope.
func (e FlightEnvelope) ClampCoord(v float64) float64 {
	return clamp(v, -e.MaxCoordXY, e.MaxCoordXY)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
