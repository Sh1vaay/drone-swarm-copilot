package core

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// Settings holds process-wide configuration.
type Settings struct {
	HTTPAddr      string
	GeminiAPIKey  string
	GeminiModel   string
	WSAuthToken   string
	WSMaxMsgBytes int64
	WSRatePerMin  int
}

// Load reads .env (if present) then environment variables.
func Load() Settings {
	return Settings{
		HTTPAddr:      envOr("HTTP_ADDR", "127.0.0.1:8080"),
		GeminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
		GeminiModel:   envOr("GEMINI_MODEL", "gemini-2.0-flash"),
		WSAuthToken:   os.Getenv("WS_AUTH_TOKEN"),
		WSMaxMsgBytes: envInt64Or("WS_MAX_MSG_BYTES", 8192),
		WSRatePerMin:  envIntOr("WS_RATE_LIMIT", 10),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("[WARN] invalid %s=%q, using %d", key, v, def)
		return def
	}
	return n
}

func envInt64Or(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Printf("[WARN] invalid %s=%q, using %d", key, v, def)
		return def
	}
	return n
}

// Validate returns an error for unusable configuration.
func (s Settings) Validate() error {
	if s.WSMaxMsgBytes < 256 || s.WSMaxMsgBytes > 1<<20 {
		return fmt.Errorf("WS_MAX_MSG_BYTES out of range: %d", s.WSMaxMsgBytes)
	}
	if s.WSRatePerMin < 1 || s.WSRatePerMin > 600 {
		return fmt.Errorf("WS_RATE_LIMIT out of range: %d", s.WSRatePerMin)
	}
	return nil
}
