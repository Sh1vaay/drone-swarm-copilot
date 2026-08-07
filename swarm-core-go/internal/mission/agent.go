package mission

import (
	"log"
	"math"

	"swarm-core-go/internal/core"
	"swarm-core-go/internal/formation"
	"swarm-core-go/internal/intelligence"
	"swarm-core-go/internal/safety"
)

// Agent represents an autonomous drone with its own decision loop.
type Agent struct {
	ID           string
	Battery      int32
	Position     core.Waypoint
	Velocity     core.Vector3D
	Acceleration core.Vector3D // Physical acceleration vector
	Mass         float64       // Drone mass in kg
	MaxThrust    float64       // Maximum force output in Newtons
	DragCoef     float64       // Aerodynamic drag coefficient
	Target       core.Waypoint
	NewMemories  []core.MemoryPoint // Local memory points generated this tick
}

// GlobalState represents the environment the agent can sense.
type GlobalState struct {
	OtherAgents       map[string]core.AgentSnapshot
	World             core.SharedWorldModel
	Intent            core.ParsedIntent
	Step              int
	DroneIndex        int
	TotalDrones       int
	ActiveDroneIndex  int
	ActiveTotalDrones int
	LeaderID          string
	MySquadID         string
	MyMissionTask     string
	Memory            []core.MemoryPoint
}

// Tick executes one cycle of the Sense -> Think -> Decide -> Act loop.
func (a *Agent) Tick(globalState GlobalState) {
	// 1. Sense
	neighbors := a.sense(globalState)

	// 2. Think
	hazard, escape := a.think(neighbors)

	// 3. Decide
	a.decide(globalState, hazard, escape)

	// 4. Act
	a.act(globalState)
}

func (a *Agent) sense(globalState GlobalState) []core.AgentSnapshot {
	var neighbors []core.AgentSnapshot
	for id, snap := range globalState.OtherAgents {
		if id != a.ID {
			neighbors = append(neighbors, snap)
		}
	}
	return neighbors
}

func (a *Agent) think(neighbors []core.AgentSnapshot) (bool, core.Vector3D) {
	speed := math.Sqrt(a.Velocity.X*a.Velocity.X + a.Velocity.Y*a.Velocity.Y + a.Velocity.Z*a.Velocity.Z)

	for _, neighbor := range neighbors {
		pos := neighbor.Position
		breach, escape := safety.VerifyProximityBreach(a.Position.X, a.Position.Y, a.Position.Z, pos.X, pos.Y, pos.Z, speed)
		if breach {
			return true, escape
		}
	}
	return false, core.Vector3D{}
}

func (a *Agent) decide(globalState GlobalState, hazard bool, escape core.Vector3D) {
	if a.Battery <= 0 {
		a.Target = core.Waypoint{
			X: a.Position.X,
			Y: a.Position.Y,
			Z: 0.05,
		}
		a.MaxThrust = 0.0
		return
	}

	if a.Battery < 15 {
		a.Target = core.Waypoint{X: a.Position.X, Y: a.Position.Y, Z: 0.05}
		return
	}

	if hazard {
		a.Target = core.Waypoint{
			X: a.Position.X + escape.X*0.5,
			Y: a.Position.Y + escape.Y*0.5,
			Z: a.Position.Z + escape.Z*0.5,
		}
		if a.Target.Z < 0.2 {
			a.Target.Z = 0.2
		}
		a.NewMemories = append(a.NewMemories, core.MemoryPoint{
			X: a.Position.X, Y: a.Position.Y, Z: a.Position.Z, Type: "DANGER",
		})
		log.Printf("[HAZARD] %s autonomous evade", a.ID)
		return
	}

	intent := globalState.Intent
	useGeminiCoords := intelligence.HasFullPositionSet(intent, globalState.TotalDrones)
	relayAlt := core.DefaultEnvelope.MaxAltitudeM

	if intent.Action == "SEARCH" || intent.Action == "SEARCH_AREA" || intent.Action == "RECON" {
		task := globalState.MyMissionTask

		focalX, focalY := 3.0, 3.0
		if useGeminiCoords && len(intent.Positions) > 0 {
			pos := intent.Positions[0]
			focalX, focalY = 3.0+pos.X, 3.0+pos.Y
		}

		if task == "RELAY" {
			a.Target = core.Waypoint{
				X: focalX,
				Y: focalY,
				Z: relayAlt,
			}
			a.Target = a.applyObstacleRepulsion(a.Target, globalState)
			return
		} else if task == "MAPPING" {
			altitude := 2.0
			radius := intent.Radius
			if radius <= 0 {
				radius = 3.0
			}

			var nearestTarget *core.Target
			minDistSq := math.MaxFloat64
			for _, t := range globalState.World.Targets {
				dx := a.Position.X - t.X
				dy := a.Position.Y - t.Y
				distSq := dx*dx + dy*dy

				if distSq < 64.0 && distSq < minDistSq {
					minDistSq = distSq
					targetCopy := t
					nearestTarget = &targetCopy
				}

				if distSq < 9.0 {
					a.NewMemories = append(a.NewMemories, core.MemoryPoint{X: t.X, Y: t.Y, Z: t.Z, Type: "TARGET"})
				}
			}

			var neighbors []core.AgentSnapshot
			for id, snap := range globalState.OtherAgents {
				if id != a.ID && snap.SquadID == globalState.MySquadID && snap.MissionTask == "MAPPING" {
					neighbors = append(neighbors, snap)
				}
			}
			a.Target = a.computeBoidsTarget(neighbors, radius, altitude)

			if nearestTarget != nil {
				dx := nearestTarget.X - a.Position.X
				dy := nearestTarget.Y - a.Position.Y
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist > 0 {
					a.Target.X += (dx / dist) * 1.5
					a.Target.Y += (dy / dist) * 1.5
				}
			}

			a.Target = a.applyObstacleRepulsion(a.Target, globalState)
			return
		} else if task == "SEARCH" {
			altitude := 5.0
			radius := intent.Radius
			if radius <= 0 {
				radius = 8.0
			}

			var neighbors []core.AgentSnapshot
			for id, snap := range globalState.OtherAgents {
				if id != a.ID && snap.SquadID == globalState.MySquadID && snap.MissionTask == "SEARCH" {
					neighbors = append(neighbors, snap)
				}
			}

			a.Target = a.computeBoidsTarget(neighbors, radius, altitude)

			for _, n := range neighbors {
				dx := a.Position.X - n.Position.X
				dy := a.Position.Y - n.Position.Y
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < 5.0 && dist > 0 {
					a.Target.X += (dx / dist) * 2.0
					a.Target.Y += (dy / dist) * 2.0
				}
			}

			a.Target = a.applyObstacleRepulsion(a.Target, globalState)
			return
		} else if task == "TRACK" {
			altitude := 3.0

			var trackedTarget *core.Target
			minDistSq := math.MaxFloat64
			for _, t := range globalState.World.Targets {
				dx := a.Position.X - t.X
				dy := a.Position.Y - t.Y
				distSq := dx*dx + dy*dy
				if distSq < 144.0 && distSq < minDistSq {
					minDistSq = distSq
					targetCopy := t
					trackedTarget = &targetCopy
				}

				if distSq < 9.0 {
					a.NewMemories = append(a.NewMemories, core.MemoryPoint{X: t.X, Y: t.Y, Z: t.Z, Type: "TARGET"})
				}
			}

			if trackedTarget != nil {
				dt := 0.1
				predictionFactor := 15.0
				interceptX := trackedTarget.X + trackedTarget.Velocity.X*dt*predictionFactor
				interceptY := trackedTarget.Y + trackedTarget.Velocity.Y*dt*predictionFactor

				a.Target = core.Waypoint{
					X: interceptX,
					Y: interceptY,
					Z: altitude,
				}

				var neighbors []core.AgentSnapshot
				for id, snap := range globalState.OtherAgents {
					if id != a.ID && snap.SquadID == globalState.MySquadID && snap.MissionTask == "TRACK" {
						neighbors = append(neighbors, snap)
					}
				}

				a.Target = a.computeBoidsTarget(neighbors, 2.0, altitude)

				dx := interceptX - a.Position.X
				dy := interceptY - a.Position.Y
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist > 0 {
					a.Target.X += (dx / dist) * 2.5
					a.Target.Y += (dy / dist) * 2.5
				}
			} else {
				a.Target = core.Waypoint{X: a.Position.X, Y: a.Position.Y, Z: altitude}
			}

			a.Target = a.applyObstacleRepulsion(a.Target, globalState)
			return
		}
	}

	// Only enter flocking mode when the intent is explicitly FLOCK.
	// A prior clause `(!isLeader && useGeminiCoords)` caused non-leader drones
	// to flock for any Gemini-coordinated intent (e.g. CIRCLE, LINE), overriding their
	// formation positions - a behavioural bug that ignored the actual intent action.
	if intent.Action == "flock" || intent.Action == "FLOCK" {
		radius := intent.Radius
		if radius <= 0 {
			radius = 3.0
		}
		altitude := intent.Altitude
		if altitude <= 0 {
			altitude = 2.5
		}

		var neighbors []core.AgentSnapshot
		for id, snap := range globalState.OtherAgents {
			if id != a.ID && snap.SquadID == globalState.MySquadID {
				neighbors = append(neighbors, snap)
			}
		}

		a.Target = a.computeBoidsTarget(neighbors, radius, altitude)
		a.Target = a.applyObstacleRepulsion(a.Target, globalState)
		return
	}

	if useGeminiCoords {
		idx := globalState.ActiveDroneIndex
		if idx < 0 || idx >= len(intent.Positions) {
			idx = 0
		}
		pos := intent.Positions[idx]
		a.Target = core.Waypoint{
			X: 3.0 + pos.X,
			Y: 3.0 + pos.Y,
			Z: pos.Z,
		}
	} else {
		radius := intent.Radius
		if radius <= 0 {
			radius = 3.0
		}
		altitude := intent.Altitude
		if altitude <= 0 {
			altitude = 2.5
		}

		offsets := formation.ComputeFormationOffsets(intent.Action, globalState.ActiveTotalDrones, radius)

		idx := globalState.ActiveDroneIndex
		if idx < 0 || idx >= len(offsets) {
			idx = 0
		}
		off := offsets[idx]

		targetZ := altitude + off.Z
		if off.Z == -99 {
			targetZ = 0.05
		}

		a.Target = core.Waypoint{
			X: 3.0 + off.X,
			Y: 3.0 + off.Y,
			Z: targetZ,
		}
	}

	a.Target = a.applyObstacleRepulsion(a.Target, globalState)
}

func (a *Agent) applyObstacleRepulsion(target core.Waypoint, globalState GlobalState) core.Waypoint {
	var obsX, obsY, obsZ float64
	for _, obs := range globalState.World.Obstacles {
		dx := a.Position.X - obs.X
		dy := a.Position.Y - obs.Y
		distSq := dx*dx + dy*dy

		if a.Position.Z < obs.Height && distSq > 0 {
			dist := math.Sqrt(distSq)
			safeRadius := obs.Radius + 1.8
			if dist < safeRadius {
				force := (safeRadius - dist) / safeRadius
				force *= force * 20.0

				obsX += (dx / dist) * force
				obsY += (dy / dist) * force
				obsZ += force * 0.5
			}
		}
	}

	for _, mem := range globalState.Memory {
		if mem.Type == "DANGER" {
			dx := a.Position.X - mem.X
			dy := a.Position.Y - mem.Y
			distSq := dx*dx + dy*dy
			if distSq > 0 && distSq < 9.0 {
				dist := math.Sqrt(distSq)
				force := (3.0 - dist) / 3.0
				force *= force * 15.0
				obsX += (dx / dist) * force
				obsY += (dy / dist) * force
			}
		}
	}

	return core.Waypoint{
		X: target.X + obsX,
		Y: target.Y + obsY,
		Z: target.Z + obsZ,
	}
}

func (a *Agent) act(globalState GlobalState) {
	dt := 0.1

	pGain := 5.0
	thrustX := (a.Target.X - a.Position.X) * pGain
	thrustY := (a.Target.Y - a.Position.Y) * pGain
	thrustZ := (a.Target.Z - a.Position.Z) * pGain

	thrustMag := math.Sqrt(thrustX*thrustX + thrustY*thrustY + thrustZ*thrustZ)
	if thrustMag > a.MaxThrust {
		scale := a.MaxThrust / thrustMag
		thrustX *= scale
		thrustY *= scale
		thrustZ *= scale
	}

	dragX := -a.DragCoef * a.Velocity.X
	dragY := -a.DragCoef * a.Velocity.Y
	dragZ := -a.DragCoef * a.Velocity.Z

	windX := globalState.World.Weather.WindSpeed * globalState.World.Weather.WindDirX * 2.0
	windY := globalState.World.Weather.WindSpeed * globalState.World.Weather.WindDirY * 2.0
	windZ := 0.0

	netForceX := thrustX + dragX + windX
	netForceY := thrustY + dragY + windY
	netForceZ := thrustZ + dragZ + windZ

	a.Acceleration.X = netForceX / a.Mass
	a.Acceleration.Y = netForceY / a.Mass
	a.Acceleration.Z = netForceZ / a.Mass

	a.Velocity.X += a.Acceleration.X * dt
	a.Velocity.Y += a.Acceleration.Y * dt
	a.Velocity.Z += a.Acceleration.Z * dt

	if a.Position.Z > 0.1 && a.Battery >= 15 {
		a.Velocity.Z += 0.5 * math.Cos(float64(globalState.Step)*0.05) * dt
	}

	newX := a.Position.X + a.Velocity.X*dt
	newY := a.Position.Y + a.Velocity.Y*dt
	newZ := a.Position.Z + a.Velocity.Z*dt

	if newZ < globalState.World.Terrain.MinAltitude {
		newZ = globalState.World.Terrain.MinAltitude
		a.Velocity.Z = 0.0
	}
	if newZ > globalState.World.Terrain.MaxAltitude {
		newZ = globalState.World.Terrain.MaxAltitude
		a.Velocity.Z = 0.0
	}

	a.Position = core.Waypoint{X: newX, Y: newY, Z: newZ}

	if globalState.Step%30 == 0 && a.Battery > 5 {
		a.Battery--
	}
}

func (a *Agent) computeBoidsTarget(neighbors []core.AgentSnapshot, radius float64, altitude float64) core.Waypoint {
	var sepX, sepY, sepZ float64
	var alignX, alignY, alignZ float64
	var cohX, cohY, cohZ float64

	perceptionRadius := 4.0
	count := 0

	for _, n := range neighbors {
		dx := a.Position.X - n.Position.X
		dy := a.Position.Y - n.Position.Y
		dz := a.Position.Z - n.Position.Z
		distSq := dx*dx + dy*dy + dz*dz

		if distSq > 0 && distSq < perceptionRadius*perceptionRadius {
			sepX += dx / distSq
			sepY += dy / distSq
			sepZ += dz / distSq

			alignX += n.Velocity.X
			alignY += n.Velocity.Y
			alignZ += n.Velocity.Z

			cohX += n.Position.X
			cohY += n.Position.Y
			cohZ += n.Position.Z

			count++
		}
	}

	if count > 0 {
		alignX /= float64(count)
		alignY /= float64(count)
		alignZ /= float64(count)

		cohX /= float64(count)
		cohY /= float64(count)
		cohZ /= float64(count)

		cohX = cohX - a.Position.X
		cohY = cohY - a.Position.Y
		cohZ = cohZ - a.Position.Z
	}

	wSep, wAlign, wCoh := 1.5, 1.0, 1.0

	vx := a.Velocity.X + (sepX * wSep) + (alignX * wAlign) + (cohX * wCoh)
	vy := a.Velocity.Y + (sepY * wSep) + (alignY * wAlign) + (cohY * wCoh)
	vz := a.Velocity.Z + (sepZ * wSep) + (alignZ * wAlign) + (cohZ * wCoh)

	worldCenter := core.Waypoint{X: 3.0, Y: 3.0, Z: altitude}
	vx += (worldCenter.X - a.Position.X) * 0.1
	vy += (worldCenter.Y - a.Position.Y) * 0.1
	vz += (worldCenter.Z - a.Position.Z) * 0.1

	vMag := math.Sqrt(vx*vx + vy*vy + vz*vz)
	if vMag > 2.0 {
		vx = (vx / vMag) * 2.0
		vy = (vy / vMag) * 2.0
		vz = (vz / vMag) * 2.0
	}

	return core.Waypoint{
		X: a.Position.X + vx,
		Y: a.Position.Y + vy,
		Z: a.Position.Z + vz,
	}
}
