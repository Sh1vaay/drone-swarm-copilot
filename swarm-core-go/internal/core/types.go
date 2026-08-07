// Package core defines central domain entities and shared configuration.
package core

type Vector3D struct {
	X float64
	Y float64
	Z float64
}

type Waypoint struct {
	X float64
	Y float64
	Z float64
}

type MockDroneState struct {
	DroneID          string
	SquadID          string
	MissionTask      string
	Battery          int32
	Alt              float64
	CommQuality      float64 // 0.0 to 1.0
	TrustScore       float64 // 0.0 to 1.0
	DistanceToCenter float64 // in meters
}

// DronePosition holds the exact 3D coordinate Gemini computes for a single drone.
type DronePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"` // altitude in metres
}

// Obstacle represents an environmental physical boundary (e.g. tree, tower)
type Obstacle struct {
	ID     string
	X      float64
	Y      float64
	Radius float64
	Height float64
}

type Weather struct {
	WindSpeed float64 `json:"wind_speed"`
	WindDirX  float64 `json:"wind_dir_x"`
	WindDirY  float64 `json:"wind_dir_y"`
}

type Target struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"` // e.g. "VEHICLE", "HUMAN", "STRUCTURE"
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Z        float64  `json:"z"`
	Velocity Vector3D `json:"velocity"`
}

type Terrain struct {
	MaxAltitude float64 `json:"max_altitude"`
	MinAltitude float64 `json:"min_altitude"`
}

type MemoryPoint struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	Type string  `json:"type"` // e.g. "DANGER", "TARGET"
}

// SharedWorldModel provides collective awareness to all agents
type SharedWorldModel struct {
	Targets   []Target   `json:"targets"`
	Obstacles []Obstacle `json:"obstacles"`
	Weather   Weather    `json:"weather"`
	Terrain   Terrain    `json:"terrain"`
}

// AgentSnapshot holds minimal memory of neighboring peers.
type AgentSnapshot struct {
	SquadID     string
	MissionTask string
	Position    Waypoint
	Velocity    Vector3D
}

// ParsedIntent holds the fully structured swarm command produced by Gemini AI.
// When Positions is non-empty, each entry maps directly to a drone by index —
// the server uses those coordinates instead of the formation solver.
type ParsedIntent struct {
	Action          string            `json:"action"`
	Target          string            `json:"target,omitempty"`
	Radius          float64           `json:"radius,omitempty"`
	Altitude        float64           `json:"altitude,omitempty"`
	Speed           float64           `json:"speed,omitempty"`
	Count           int               `json:"count,omitempty"`
	Positions       []DronePosition   `json:"positions,omitempty"`        // Gemini-computed coordinates
	RoleAssignments map[string]string `json:"role_assignments,omitempty"` // Gemini-computed tactical roles
	PlanDescription string            `json:"plan_description,omitempty"` // AI explanation of the strategy
}
