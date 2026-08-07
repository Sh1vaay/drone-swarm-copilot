package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"swarm-core-go/internal/core"
	"swarm-core-go/internal/mission"
	"swarm-core-go/internal/transport/grpc"
)

var practiceObstacles = []core.Obstacle{
	{ID: "Tree-1", X: -7, Y: -6, Radius: 1.0, Height: 5.0},
	{ID: "Tree-2", X: 6, Y: -7, Radius: 1.0, Height: 5.0},
	{ID: "Tree-3", X: -5, Y: 7, Radius: 1.0, Height: 5.0},
	{ID: "Tree-4", X: 7, Y: 5, Radius: 1.0, Height: 5.0},
	{ID: "Tree-5", X: -8, Y: 1, Radius: 1.0, Height: 5.0},
	{ID: "Tree-6", X: 5, Y: -4, Radius: 1.0, Height: 5.0},
	{ID: "Tree-7", X: -3, Y: -7, Radius: 1.0, Height: 5.0},
	{ID: "Tree-8", X: 8, Y: -2, Radius: 1.0, Height: 5.0},
	{ID: "Tree-9", X: -6, Y: 6, Radius: 1.0, Height: 5.0},
	{ID: "Tree-10", X: 3, Y: 8, Radius: 1.0, Height: 5.0},
	{ID: "Tower-1", X: -6, Y: 5, Radius: 0.8, Height: 10.0},
	{ID: "Tower-2", X: 7, Y: -3, Radius: 0.8, Height: 10.0},
	{ID: "Tower-3", X: 1, Y: -8, Radius: 0.8, Height: 10.0},
	{ID: "Tower-4", X: -5, Y: -5, Radius: 0.8, Height: 10.0},
	{ID: "Barrels-1", X: -4, Y: 4, Radius: 1.5, Height: 1.5},
	{ID: "Barrels-2", X: 5, Y: -5, Radius: 1.5, Height: 1.5},
	{ID: "Barrels-3", X: -2, Y: 6, Radius: 1.5, Height: 1.5},
	{ID: "Barrels-4", X: 6, Y: 6, Radius: 1.5, Height: 1.5},
}

var (
	sharedWorld   core.SharedWorldModel
	sharedWorldMu sync.RWMutex
)

var droneIDs = []string{
	"drone-sim-A", "drone-sim-B", "drone-sim-C", "drone-sim-D",
	"drone-sim-E", "drone-sim-F", "drone-sim-G", "drone-sim-H",
	"drone-sim-I", "drone-sim-J", "drone-sim-K", "drone-sim-L",
	"drone-sim-M", "drone-sim-N", "drone-sim-O", "drone-sim-P",
}

func init() {
	env := core.DefaultEnvelope
	sharedWorld = core.SharedWorldModel{
		Obstacles: practiceObstacles,
		Targets: []core.Target{
			{ID: "Vehicle-Alpha", Type: "VEHICLE", X: -10, Y: -10, Z: 0.0, Velocity: core.Vector3D{X: 1.5}},
			{ID: "Human-Bravo", Type: "HUMAN", X: 12, Y: 8, Z: 0.0, Velocity: core.Vector3D{Y: -0.5}},
			{ID: "Outpost-Charlie", Type: "STRUCTURE", X: 0, Y: 15, Z: 0.0},
		},
		Weather: core.Weather{WindSpeed: 0.5, WindDirX: 1.0, WindDirY: 0.2},
		// MinGroundZ (0.05m) is the absolute physical floor used by the agent physics engine.
		// MinAltitudeM (0.20m) is the sustained-flight hover target, not the coordinate floor -
		// using MinAltitudeM here would prevent the physics clamp from ever letting drones land.
		Terrain: core.Terrain{MinAltitude: env.MinGroundZ, MaxAltitude: env.MaxAltitudeM},
	}
}

func main() {
	_ = godotenv.Load()
	cfg := core.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	if cfg.GeminiAPIKey != "" {
		_ = os.Setenv("GEMINI_API_KEY", cfg.GeminiAPIKey)
	}
	if cfg.GeminiModel != "" {
		_ = os.Setenv("GEMINI_MODEL", cfg.GeminiModel)
	}

	grpc.ConfigureWebSocket(grpc.WSSettings{
		AuthToken:   cfg.WSAuthToken,
		MaxMsgBytes: cfg.WSMaxMsgBytes,
		RatePerMin:  cfg.WSRatePerMin,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/client", grpc.ServeWebSocket)
	mux.HandleFunc("/", serveHUD)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				grpc.Swarm.RunPeriodicElection()
			}
		}
	}()

	go runDroneSimulation(ctx)

	log.Printf("Swarm Intelligence Core listening on %s", cfg.HTTPAddr)
	log.Printf("Open HUD: http://%s/", cfg.HTTPAddr)
	if cfg.WSAuthToken != "" {
		log.Println("WebSocket auth: Bearer token required")
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func serveHUD(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self' https://fonts.googleapis.com https://fonts.gstatic.com "+
			"https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; "+
			"script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; "+
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
			"connect-src 'self' ws: wss:; img-src 'self' data:;")
	http.ServeFile(w, r, "web/hud.html")
}

func runDroneSimulation(ctx context.Context) {
	time.Sleep(time.Second)
	log.Printf("Drone simulation active for %d drones", len(droneIDs))

	agents := make([]*mission.Agent, len(droneIDs))
	for i, id := range droneIDs {
		bat := int32(90 - i*2)
		if bat < 20 {
			bat = 20
		}
		angle := (2.0 * math.Pi * float64(i)) / float64(len(droneIDs))
		start := core.Waypoint{X: 3.0 + 3.5*math.Cos(angle), Y: 3.0 + 3.5*math.Sin(angle), Z: 0.05}
		agents[i] = &mission.Agent{
			ID: id, Battery: bat, Position: start, Target: start,
			Mass: 1.5, MaxThrust: 30.0, DragCoef: 1.0,
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	step := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			step++
			intent := grpc.Swarm.GetLastParsedIntent()
			advanceWorldTargets(0.1)

			sharedWorldMu.RLock()
			world := sharedWorld
			sharedWorldMu.RUnlock()

			otherPositions := make(map[string]core.AgentSnapshot, len(droneIDs))
			activeIndices := make(map[string]int)
			activeCount := 0
			for _, a := range agents {
				if a.Battery > 0 {
					activeIndices[a.ID] = activeCount
					activeCount++
				}
				sx, sy, sz := int(a.Position.X/15), int(a.Position.Y/15), int(a.Position.Z/15)
				otherPositions[a.ID] = core.AgentSnapshot{
					SquadID:     squadID(sx, sy, sz),
					MissionTask: grpc.Swarm.GetMissionTask(a.ID),
					Position:    a.Position,
					Velocity:    a.Velocity,
				}
			}

			for i, a := range agents {
				sx, sy, sz := int(a.Position.X/15), int(a.Position.Y/15), int(a.Position.Z/15)
				mySquad := squadID(sx, sy, sz)
				activeIdx := -1
				if a.Battery > 0 {
					activeIdx = activeIndices[a.ID]
				}
				globalState := mission.GlobalState{
					OtherAgents: otherPositions, World: world, Intent: intent, Step: step,
					DroneIndex: i, TotalDrones: len(droneIDs), ActiveDroneIndex: activeIdx,
					ActiveTotalDrones: activeCount, MySquadID: mySquad,
					LeaderID:      grpc.Swarm.GetSquadLeader(mySquad),
					MyMissionTask: grpc.Swarm.GetMissionTask(a.ID),
					Memory:        grpc.Swarm.RecallMemory(),
				}
				a.Tick(globalState)
				for _, mem := range a.NewMemories {
					grpc.Swarm.RecordMemory(mem.X, mem.Y, mem.Z, mem.Type)
				}
				a.NewMemories = nil
				status, escape := grpc.StreamHandler(a.ID, a.Position.X, a.Position.Y, a.Position.Z, a.Battery)
				if status == "COLLISION_HAZARD" {
					log.Printf("[HAZARD] %s proximity breach escape Z=%.2f", a.ID, escape.Z)
				}
			}

			if step%100 == 0 {
				log.Printf("[SIM] step=%d intent=%s", step, intent.Action)
			}
		}
	}
}

func squadID(x, y, z int) string {
	return fmt.Sprintf("SQUAD-%d-%d-%d", x, y, z)
}

func advanceWorldTargets(dt float64) {
	sharedWorldMu.Lock()
	defer sharedWorldMu.Unlock()
	for i := range sharedWorld.Targets {
		sharedWorld.Targets[i].X += sharedWorld.Targets[i].Velocity.X * dt
		sharedWorld.Targets[i].Y += sharedWorld.Targets[i].Velocity.Y * dt
		sharedWorld.Targets[i].Z += sharedWorld.Targets[i].Velocity.Z * dt
	}
}
