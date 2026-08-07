package grpc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"swarm-core-go/internal/core"
	"swarm-core-go/internal/intelligence"

	"github.com/gorilla/websocket"
)

var (
	upgrader websocket.Upgrader
	wsCfg    WSSettings
)

// WSSettings configures the WebSocket command channel.
type WSSettings struct {
	AuthToken   string
	MaxMsgBytes int64
	RatePerMin  int
}

func init() {
	ConfigureWebSocket(WSSettings{MaxMsgBytes: 8192, RatePerMin: 10})
}

// ConfigureWebSocket applies security-related WebSocket settings.
func ConfigureWebSocket(cfg WSSettings) {
	wsCfg = cfg
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkWebSocketOrigin,
	}
}

func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin != "" {
		return strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "https://localhost") ||
			strings.HasPrefix(origin, "https://127.0.0.1")
	}
	return isLoopbackRemoteAddr(r.RemoteAddr)
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type ClientRegistry struct {
	sync.Mutex
	clients map[*websocket.Conn]bool
}

var Registry = &ClientRegistry{clients: make(map[*websocket.Conn]bool)}

func (cr *ClientRegistry) Register(conn *websocket.Conn) {
	cr.Lock()
	defer cr.Unlock()
	cr.clients[conn] = true
}

func (cr *ClientRegistry) Deregister(conn *websocket.Conn) {
	cr.Lock()
	defer cr.Unlock()
	delete(cr.clients, conn)
}

func (cr *ClientRegistry) Broadcast(payload interface{}) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Broadcast marshal error: %v", err)
		return
	}

	cr.Lock()
	defer cr.Unlock()
	for client := range cr.clients {
		_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := client.WriteMessage(websocket.TextMessage, bytes); err != nil {
			log.Printf("WS write failed: %v", err)
			_ = client.Close()
			delete(cr.clients, client)
		}
	}
}

// IntentBroadcast carries the Gemini-parsed intent back to all HUD clients.
type IntentBroadcast struct {
	Type      string               `json:"type"`
	Action    string               `json:"action"`
	Target    string               `json:"target"`
	Raw       string               `json:"raw"`
	Radius    float64              `json:"radius"`
	Altitude  float64              `json:"altitude"`
	Speed     float64              `json:"speed"`
	Count     int                  `json:"count"`
	Positions []core.DronePosition `json:"positions,omitempty"`
}

type rateLimiter struct {
	mu     sync.Mutex
	window time.Time
	count  int
	limit  int
}

func newRateLimiter(perMin int) *rateLimiter {
	return &rateLimiter{limit: perMin, window: time.Now()}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.window) > time.Minute {
		rl.window = now
		rl.count = 0
	}
	if rl.count >= rl.limit {
		return false
	}
	rl.count++
	return true
}

func authorizeWS(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	providedToken := strings.TrimSpace(auth[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(token)) == 1
}

func effectiveMaxMsgBytes() int64 {
	if wsCfg.MaxMsgBytes > 0 {
		return wsCfg.MaxMsgBytes
	}
	return 8192
}

func effectiveRatePerMin() int {
	if wsCfg.RatePerMin > 0 {
		return wsCfg.RatePerMin
	}
	return 10
}

// ServeWebSocket handles operator HUD WebSocket streaming.
func ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	if !authorizeWS(r, wsCfg.AuthToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	maxBytes := effectiveMaxMsgBytes()
	conn.SetReadLimit(maxBytes)

	Registry.Register(conn)
	Registry.Lock()
	active := len(Registry.clients)
	Registry.Unlock()
	log.Printf("WebHUD operator connected from %s (active: %d)", r.RemoteAddr, active)

	limiter := newRateLimiter(effectiveRatePerMin())

	defer func() {
		Registry.Deregister(conn)
		_ = conn.Close()
	}()

	type CommandMessage struct {
		Command string `json:"command"`
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebHUD connection closed: %v", err)
			break
		}
		if int64(len(msg)) > maxBytes {
			log.Printf("WebHUD message too large: %d bytes", len(msg))
			continue
		}

		var cmd CommandMessage
		if err := json.Unmarshal(msg, &cmd); err != nil || cmd.Command == "" {
			continue
		}

		if !limiter.allow() {
			log.Printf("WebHUD rate limit exceeded from %s", r.RemoteAddr)
			continue
		}

		lowerCmd := strings.ToLower(strings.TrimSpace(cmd.Command))
		if lowerCmd == "mode manual" || lowerCmd == "manual mode" {
			Swarm.SetStrategyMode("MANUAL") // guarded by SwarmManager mutex
			log.Println("Strategy mode: MANUAL")
			continue
		}
		if lowerCmd == "mode auto" || lowerCmd == "auto mode" {
			Swarm.SetStrategyMode("AUTO") // guarded by SwarmManager mutex
			log.Println("Strategy mode: AUTO")
			continue
		}

		if Swarm.GetStrategyMode() == "MANUAL" {
			if lowerCmd == "approve" || lowerCmd == "execute plan" || lowerCmd == "execute" {
				pending := Swarm.GetPendingIntent()
				if pending != nil {
					Swarm.SetLastParsedIntent(*pending)
					intent := *pending
					Registry.Broadcast(IntentBroadcast{
						Type: "intent_update", Action: intent.Action, Target: intent.Target,
						Raw: "Approved plan", Radius: intent.Radius, Altitude: intent.Altitude,
						Speed: intent.Speed, Count: intent.Count, Positions: intent.Positions,
					})
					log.Printf("Plan approved: %s", intelligence.RedactForLog(intent))
				}
				continue
			}
			if lowerCmd == "reject" || lowerCmd == "cancel plan" || lowerCmd == "cancel" {
				Swarm.ClearPendingIntent()
				log.Println("Plan rejected")
				continue
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		telemetryData := Swarm.GetTelemetrySnapshotJSON()
		intent := intelligence.ParseIntentWithGemini(ctx, cmd.Command, telemetryData)
		cancel()
		log.Printf("Gemini intent: %s", intelligence.RedactForLog(intent))

		if Swarm.GetStrategyMode() == "MANUAL" {
			Swarm.SetPendingIntent(intent)
			Registry.Broadcast(IntentBroadcast{
				Type: "intent_update", Action: "HOLD",
				Raw: "PENDING APPROVAL",
			})
			continue
		}

		Swarm.SetLastParsedIntent(intent)
		Registry.Broadcast(IntentBroadcast{
			Type: "intent_update", Action: intent.Action, Target: intent.Target,
			Raw: cmd.Command, Radius: intent.Radius, Altitude: intent.Altitude,
			Speed: intent.Speed, Count: intent.Count, Positions: intent.Positions,
		})
	}
}
