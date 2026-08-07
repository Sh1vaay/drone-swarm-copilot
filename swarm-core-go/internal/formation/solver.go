package formation

import (
	"math"
	"math/rand"

	"swarm-core-go/internal/core"
)

// SmoothCoordinate applies an exponential decay smoothing filter to a 1D trajectory (BR-02)
func SmoothCoordinate(current, target, alpha float64) float64 {
	return (current * alpha) + (target * (1.0 - alpha))
}

// SmoothWaypoint applies the exponential decay filter across a 3D Waypoint
func SmoothWaypoint(current, target core.Waypoint, alpha float64) core.Waypoint {
	return core.Waypoint{
		X: SmoothCoordinate(current.X, target.X, alpha),
		Y: SmoothCoordinate(current.Y, target.Y, alpha),
		Z: SmoothCoordinate(current.Z, target.Z, alpha),
	}
}

// ComputeFormationOffsets returns target 3D coordinate offsets for N drones.
// All formations are generative — they work for any drone count.
// shape is case-insensitive and maps to Gemini action names.
func ComputeFormationOffsets(shape string, numDrones int, radius float64) []core.Waypoint {
	offsets := make([]core.Waypoint, numDrones)
	if numDrones <= 0 {
		return offsets
	}

	switch shape {
	case "start", "START":
		for i := 0; i < numDrones; i++ {
			angle := (2.0 * math.Pi * float64(i)) / float64(numDrones)
			offsets[i] = core.Waypoint{
				X: radius * math.Cos(angle),
				Y: radius * math.Sin(angle),
				Z: 0,
			}
		}

	case "land", "LAND":
		for i := 0; i < numDrones; i++ {
			angle := (2.0 * math.Pi * float64(i)) / float64(numDrones)
			offsets[i] = core.Waypoint{
				X: (radius * 0.25) * math.Cos(angle),
				Y: (radius * 0.25) * math.Sin(angle),
				Z: -99,
			}
		}

	case "circle", "CIRCLE":
		for i := 0; i < numDrones; i++ {
			angle := (2.0 * math.Pi * float64(i)) / float64(numDrones)
			offsets[i] = core.Waypoint{
				X: radius * math.Cos(angle),
				Y: radius * math.Sin(angle),
				Z: 0.0,
			}
		}

	case "square", "SQUARE":
		perSide := numDrones / 4
		remainder := numDrones % 4
		idx := 0
		for side := 0; side < 4; side++ {
			count := perSide
			if side < remainder {
				count++
			}
			for j := 0; j < count; j++ {
				t := float64(j) / math.Max(float64(count), 1)
				switch side {
				case 0:
					offsets[idx] = core.Waypoint{X: -radius + 2*radius*t, Y: -radius, Z: 0}
				case 1:
					offsets[idx] = core.Waypoint{X: radius, Y: -radius + 2*radius*t, Z: 0}
				case 2:
					offsets[idx] = core.Waypoint{X: radius - 2*radius*t, Y: radius, Z: 0}
				case 3:
					offsets[idx] = core.Waypoint{X: -radius, Y: radius - 2*radius*t, Z: 0}
				}
				idx++
			}
		}

	case "triangle", "TRIANGLE":
		for i := 0; i < numDrones; i++ {
			seg := float64(i) / float64(numDrones)
			vertex := int(seg * 3)
			t := (seg * 3) - float64(vertex)
			var ax, ay, bx, by float64
			switch vertex {
			case 0:
				ax, ay = -radius, -radius*0.577
				bx, by = radius, -radius*0.577
			case 1:
				ax, ay = radius, -radius*0.577
				bx, by = 0, radius*1.155
			default:
				ax, ay = 0, radius*1.155
				bx, by = -radius, -radius*0.577
			}
			offsets[i] = core.Waypoint{X: ax + (bx-ax)*t, Y: ay + (by-ay)*t, Z: 0}
		}

	case "v_shape", "V_SHAPE":
		offsets[0] = core.Waypoint{X: 0, Y: 0, Z: 0}
		leftBranch := true
		branchIdx := 1
		for i := 1; i < numDrones; i++ {
			dist := float64(branchIdx) * (radius / 2.0)
			if leftBranch {
				offsets[i] = core.Waypoint{X: -dist * math.Cos(math.Pi/5), Y: -dist, Z: 0}
				leftBranch = false
			} else {
				offsets[i] = core.Waypoint{X: dist * math.Cos(math.Pi/5), Y: -dist, Z: 0}
				leftBranch = true
				branchIdx++
			}
		}

	case "diamond", "DIAMOND":
		for i := 0; i < numDrones; i++ {
			angle := (2.0*math.Pi*float64(i))/float64(numDrones) + math.Pi/4
			offsets[i] = core.Waypoint{
				X: radius * math.Cos(angle),
				Y: radius * math.Sin(angle),
				Z: 0,
			}
		}

	case "line", "LINE":
		spacing := radius * 2.0 / math.Max(float64(numDrones-1), 1)
		for i := 0; i < numDrones; i++ {
			offsets[i] = core.Waypoint{
				X: -radius + float64(i)*spacing,
				Y: 0.0,
				Z: 0.0,
			}
		}

	case "scatter", "SCATTER":
		goldenAngle := math.Pi * (3.0 - math.Sqrt(5.0))
		for i := 0; i < numDrones; i++ {
			r := radius * math.Sqrt(float64(i+1)/float64(numDrones))
			angle := float64(i) * goldenAngle
			offsets[i] = core.Waypoint{
				X: r * math.Cos(angle),
				Y: r * math.Sin(angle),
				Z: float64(i) * 0.3,
			}
		}

	case "sphere", "SPHERE":
		phi := math.Pi * (3.0 - math.Sqrt(5.0))
		for i := 0; i < numDrones; i++ {
			y := 1.0 - (float64(i)/math.Max(float64(numDrones-1), 1.0))*2.0
			r := math.Sqrt(1.0 - y*y) * radius
			theta := phi * float64(i)
			offsets[i] = core.Waypoint{
				X: math.Cos(theta) * r,
				Y: y * radius,
				Z: math.Sin(theta) * r,
			}
		}

	case "cube", "CUBE":
		for i := 0; i < numDrones; i++ {
			t := float64(i) / float64(numDrones)
			face := i % 6
			u := math.Mod(t*7.0, 1.0)*2.0 - 1.0
			v := math.Mod(t*13.0, 1.0)*2.0 - 1.0
			x, y, z := 0.0, 0.0, 0.0
			switch face {
			case 0: x, y, z = 1, u, v
			case 1: x, y, z = -1, u, v
			case 2: x, y, z = u, 1, v
			case 3: x, y, z = u, -1, v
			case 4: x, y, z = u, v, 1
			case 5: x, y, z = u, v, -1
			}
			offsets[i] = core.Waypoint{X: x * radius, Y: y * radius, Z: z * radius}
		}

	case "pyramid", "PYRAMID":
		offsets[0] = core.Waypoint{X: 0, Y: 0, Z: radius}
		for i := 1; i < numDrones; i++ {
			t := float64(i) / float64(numDrones-1)
			angle := t * 2 * math.Pi
			cos, sin := math.Cos(angle), math.Sin(angle)
			max := math.Max(math.Abs(cos), math.Abs(sin))
			x := (cos / max) * radius
			y := (sin / max) * radius
			zOffset := 0.0
			if i%3 == 0 {
				zOffset = radius * 0.5
				x *= 0.5
				y *= 0.5
			}
			offsets[i] = core.Waypoint{X: x, Y: y, Z: -radius*0.5 + zOffset}
		}

	case "helix", "HELIX":
		for i := 0; i < numDrones; i++ {
			t := float64(i) / float64(numDrones)
			angle := t * 6 * math.Pi
			offsets[i] = core.Waypoint{
				X: radius * math.Cos(angle),
				Y: radius * math.Sin(angle),
				Z: (t - 0.5) * radius * 2.0,
			}
		}

	case "spiral", "SPIRAL":
		for i := 0; i < numDrones; i++ {
			t := float64(i) / float64(numDrones)
			angle := t * 4 * math.Pi
			r := t * radius
			offsets[i] = core.Waypoint{
				X: r * math.Cos(angle),
				Y: r * math.Sin(angle),
				Z: t * 2.0,
			}
		}

	case "spread", "SPREAD":
		for i := 0; i < numDrones; i++ {
			angle := (2.0 * math.Pi * float64(i)) / float64(numDrones)
			offsets[i] = core.Waypoint{
				X: radius * math.Cos(angle),
				Y: radius * math.Sin(angle),
				Z: float64(i) * 0.2,
			}
		}

	case "random_scatter":
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < numDrones; i++ {
			offsets[i] = core.Waypoint{
				X: (rng.Float64()*2 - 1) * radius,
				Y: (rng.Float64()*2 - 1) * radius,
				Z: rng.Float64() * 1.5,
			}
		}

	default:
		for i := 0; i < numDrones; i++ {
			angle := (2.0 * math.Pi * float64(i)) / float64(numDrones)
			offsets[i] = core.Waypoint{
				X: (radius * 0.8) * math.Cos(angle),
				Y: (radius * 0.8) * math.Sin(angle),
				Z: 0,
			}
		}
	}

	return offsets
}
