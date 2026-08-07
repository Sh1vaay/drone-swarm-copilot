package grpc

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"swarm-core-go/internal/core"
	"swarm-core-go/internal/formation"
	"swarm-core-go/internal/safety"
)

const maxMemoryPoints = 500

// ActiveDrone represents a tracked simulated drone inside Go safety memory.
type ActiveDrone struct {
	DroneID           string    `json:"drone_id"`
	Timestamp         int64     `json:"timestamp"`
	X                 float64   `json:"x"`
	Y                 float64   `json:"y"`
	Z                 float64   `json:"z"`
	Pitch             float32   `json:"pitch"`
	Roll              float32   `json:"roll"`
	Yaw               float32   `json:"yaw"`
	Battery           int32     `json:"battery"`
	Role              string    `json:"role"`
	MissionTask       string    `json:"mission_task"`
	SquadID           string    `json:"squad_id"`
	TargetReferenceID string    `json:"target_ref_id"`
	CommQuality       float64   `json:"comm_quality"`
	TrustScore        float64   `json:"trust_score"`
	LastSeen          time.Time `json:"last_seen"`
}

// SwarmManager coordinates in-memory swarm state.
type SwarmManager struct {
	sync.RWMutex
	drones           map[string]ActiveDrone
	squadLeaders     map[string]string
	lastParsedIntent core.ParsedIntent
	pendingIntent    *core.ParsedIntent
	memoryPoints     []core.MemoryPoint
	StrategyMode     string
}

var Swarm = &SwarmManager{
	drones:           make(map[string]ActiveDrone),
	squadLeaders:     make(map[string]string),
	lastParsedIntent: core.ParsedIntent{Action: "HOLD", Target: "idle-sector"},
	memoryPoints:     make([]core.MemoryPoint, 0),
	StrategyMode:     "AUTO",
}

func (sm *SwarmManager) GetSquadLeader(squadID string) string {
	sm.RLock()
	defer sm.RUnlock()
	return sm.squadLeaders[squadID]
}

func (sm *SwarmManager) RecordMemory(x, y, z float64, pType string) {
	sm.Lock()
	defer sm.Unlock()

	for _, p := range sm.memoryPoints {
		dx, dy, dz := p.X-x, p.Y-y, p.Z-z
		if p.Type == pType && (dx*dx+dy*dy+dz*dz) < 1.0 {
			return
		}
	}

	sm.memoryPoints = append(sm.memoryPoints, core.MemoryPoint{X: x, Y: y, Z: z, Type: pType})
	if len(sm.memoryPoints) > maxMemoryPoints {
		sm.memoryPoints = sm.memoryPoints[len(sm.memoryPoints)-maxMemoryPoints:]
	}
}

func (sm *SwarmManager) RecallMemory() []core.MemoryPoint {
	sm.RLock()
	defer sm.RUnlock()
	pts := make([]core.MemoryPoint, len(sm.memoryPoints))
	copy(pts, sm.memoryPoints)
	return pts
}

func (sm *SwarmManager) SetLastParsedIntent(intent core.ParsedIntent) {
	sm.Lock()
	defer sm.Unlock()
	sm.lastParsedIntent = intent
	sm.pendingIntent = nil
}

func (sm *SwarmManager) GetLastParsedIntent() core.ParsedIntent {
	sm.RLock()
	defer sm.RUnlock()
	return sm.lastParsedIntent
}

func (sm *SwarmManager) lastParsedIntentLocked() core.ParsedIntent {
	return sm.lastParsedIntent
}

// SetStrategyMode updates StrategyMode under the write lock, preventing the
// data race between ServeWebSocket (writer) and RunPeriodicElection (reader).
func (sm *SwarmManager) SetStrategyMode(mode string) {
	sm.Lock()
	defer sm.Unlock()
	sm.StrategyMode = mode
}

// GetStrategyMode reads StrategyMode under the read lock.
func (sm *SwarmManager) GetStrategyMode() string {
	sm.RLock()
	defer sm.RUnlock()
	return sm.StrategyMode
}

func (sm *SwarmManager) SetPendingIntent(intent core.ParsedIntent) {
	sm.Lock()
	defer sm.Unlock()
	sm.pendingIntent = &intent
}

func (sm *SwarmManager) GetPendingIntent() *core.ParsedIntent {
	sm.RLock()
	defer sm.RUnlock()
	return sm.pendingIntent
}

func (sm *SwarmManager) ClearPendingIntent() {
	sm.Lock()
	defer sm.Unlock()
	sm.pendingIntent = nil
}

func (sm *SwarmManager) GetMissionTask(droneID string) string {
	sm.RLock()
	defer sm.RUnlock()
	if drone, ok := sm.drones[droneID]; ok {
		return drone.MissionTask
	}
	return "TRACK"
}

// RunPeriodicElection evaluates and updates dynamic role assignments at 2 Hz.
func (sm *SwarmManager) RunPeriodicElection() {
	sm.Lock()
	defer sm.Unlock()

	if len(sm.drones) == 0 {
		return
	}

	squadStates := make(map[string][]core.MockDroneState)
	squadCenters := make(map[string]core.Vector3D)
	squadCounts := make(map[string]float64)

	for _, d := range sm.drones {
		if time.Since(d.LastSeen) < 3*time.Second {
			center := squadCenters[d.SquadID]
			center.X += d.X
			center.Y += d.Y
			center.Z += d.Z
			squadCenters[d.SquadID] = center
			squadCounts[d.SquadID]++
		}
	}

	for squadID, count := range squadCounts {
		if count > 0 {
			center := squadCenters[squadID]
			center.X /= count
			center.Y /= count
			center.Z /= count
			squadCenters[squadID] = center
		}
	}

	for _, d := range sm.drones {
		if time.Since(d.LastSeen) < 3*time.Second {
			center := squadCenters[d.SquadID]
			dx, dy, dz := d.X-center.X, d.Y-center.Y, d.Z-center.Z
			squadStates[d.SquadID] = append(squadStates[d.SquadID], core.MockDroneState{
				DroneID:          d.DroneID,
				SquadID:          d.SquadID,
				Battery:          d.Battery,
				Alt:              d.Z,
				CommQuality:      d.CommQuality,
				TrustScore:       d.TrustScore,
				DistanceToCenter: dx*dx + dy*dy + dz*dz,
			})
		}
	}

	for squadID, states := range squadStates {
		if len(states) == 0 {
			continue
		}
		candidateLeader := formation.ElectSwarmLeader(states)
		if candidateLeader == "" {
			continue
		}

		currentLeader := sm.squadLeaders[squadID]
		var currentBattery, candidateBattery int32 = 100, 100
		if cur, ok := sm.drones[currentLeader]; ok {
			currentBattery = cur.Battery
		}
		if cand, ok := sm.drones[candidateLeader]; ok {
			candidateBattery = cand.Battery
		}

		if currentLeader == "" || formation.EvaluateHandoverNecessity(currentLeader, currentBattery, candidateLeader, candidateBattery) {
			sm.squadLeaders[squadID] = candidateLeader
			if currentLeader != candidateLeader {
				log.Printf("SQUAD [%s] handover: %s -> %s", squadID, currentLeader, candidateLeader)
			}
		}
	}

	intent := sm.lastParsedIntentLocked()
	// StrategyMode is read here while holding sm.Lock(), consistent with
	// SetStrategyMode which also acquires the lock before writing.
	mode := sm.StrategyMode
	squadTasks := make(map[string]map[string]string)
	for squadID, states := range squadStates {
		if mode == "AUTO" && len(intent.RoleAssignments) > 0 {
			squadTasks[squadID] = normalizeRoleAssignments(intent.RoleAssignments, states)
		} else {
			squadTasks[squadID] = formation.AllocateMissionTasks(intent.Action, states)
		}
	}

	for id, drone := range sm.drones {
		leaderID := sm.squadLeaders[drone.SquadID]
		if id == leaderID {
			drone.Role = "leader"
			drone.TargetReferenceID = id
		} else {
			drone.Role = "follower"
			drone.TargetReferenceID = leaderID
		}
		if taskMap, ok := squadTasks[drone.SquadID]; ok {
			if task, exists := taskMap[id]; exists {
				drone.MissionTask = task
			} else {
				drone.MissionTask = "TRACK"
			}
		} else {
			drone.MissionTask = "TRACK"
		}
		sm.drones[id] = drone
	}
}

func normalizeRoleAssignments(raw map[string]string, states []core.MockDroneState) map[string]string {
	out := make(map[string]string, len(states))
	byID := make(map[string]struct{}, len(states))
	for _, s := range states {
		byID[s.DroneID] = struct{}{}
	}
	for id, role := range raw {
		if _, ok := byID[id]; ok {
			out[id] = role
		}
	}
	if len(out) == 0 && len(raw) > 0 {
		i := 0
		roles := make([]string, 0, len(raw))
		for _, role := range raw {
			roles = append(roles, role)
		}
		for _, s := range states {
			if i < len(roles) {
				out[s.DroneID] = roles[i]
				i++
			}
		}
	}
	return out
}

func (sm *SwarmManager) TrackAndValidate(droneID string, x, y, z float64, pitch, roll, yaw float32, battery int32) (string, core.Vector3D) {
	sm.Lock()

	safe, boundaryErr := safety.CheckFlightBoundaries(z)
	if !safe {
		sm.Unlock()
		return fmt.Sprintf("CRITICAL_BOUNDARY: %s", boundaryErr), core.Vector3D{}
	}

	targetX, targetY, targetZ := x, y, z
	squadX := int(targetX / 15.0)
	squadY := int(targetY / 15.0)
	squadZ := int(targetZ / 15.0)
	squadID := fmt.Sprintf("SQUAD-%d-%d-%d", squadX, squadY, squadZ)

	existingDrone, exists := sm.drones[droneID]
	leaderID := sm.squadLeaders[squadID]
	role := "follower"
	if leaderID == droneID {
		role = "leader"
	}

	missionTask := "TRACK"
	if exists {
		missionTask = existingDrone.MissionTask
	}

	if exists && role == "follower" && leaderID != "" {
		targetX = formation.SmoothCoordinate(existingDrone.X, x, 0.8)
		targetY = formation.SmoothCoordinate(existingDrone.Y, y, 0.8)
		targetZ = formation.SmoothCoordinate(existingDrone.Z, z, 0.8)
	}

	currentDrone := ActiveDrone{
		DroneID:           droneID,
		Timestamp:         time.Now().UnixMilli(),
		X:                 targetX,
		Y:                 targetY,
		Z:                 targetZ,
		Pitch:             pitch,
		Roll:              roll,
		Yaw:               yaw,
		Battery:           battery,
		Role:              role,
		MissionTask:       missionTask,
		SquadID:           squadID,
		TargetReferenceID: leaderID,
		CommQuality:       1.0 - (float64(battery%5) * 0.02),
		TrustScore:        1.0 - (float64(battery%3) * 0.01),
		LastSeen:          time.Now(),
	}
	sm.drones[droneID] = currentDrone

	var hazard string
	var escape core.Vector3D
	for id, neighbor := range sm.drones {
		if id == droneID || neighbor.SquadID != squadID {
			continue
		}
		breached, esc := safety.VerifyProximityBreach(targetX, targetY, targetZ, neighbor.X, neighbor.Y, neighbor.Z, 3.0)
		if breached {
			hazard = "COLLISION_HAZARD"
			escape = esc
			break
		}
	}
	sm.Unlock()

	Registry.Broadcast(currentDrone)
	return coalesceStatus(hazard), escape
}

func coalesceStatus(hazard string) string {
	if hazard == "" {
		return "SAFE"
	}
	return hazard
}

func StreamHandler(droneID string, x, y, z float64, battery int32) (string, core.Vector3D) {
	return Swarm.TrackAndValidate(droneID, x, y, z, 0, 0, 0, battery)
}

func (sm *SwarmManager) GetTelemetrySnapshotJSON() string {
	sm.RLock()
	defer sm.RUnlock()

	type snapshot struct {
		DroneID string `json:"drone_id"`
		Battery int32  `json:"battery"`
	}

	snaps := make([]snapshot, 0, len(sm.drones))
	for _, d := range sm.drones {
		if d.Battery > 0 {
			snaps = append(snaps, snapshot{DroneID: d.DroneID, Battery: d.Battery})
		}
	}
	bytes, err := json.Marshal(snaps)
	if err != nil {
		return "[]"
	}
	return string(bytes)
}
