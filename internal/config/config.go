package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Duration wraps time.Duration for JSON unmarshaling (e.g. "24h", "10m").
type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d Duration) ToDuration() time.Duration { return time.Duration(d) }

type ServerConfig struct {
	IP           string   `json:"ip"`
	Port         int      `json:"port"`
	TLS          bool     `json:"tls"`
	IdleTimeout  Duration `json:"idle_timeout"`
}

type WebConfig struct {
	Addr      string `json:"addr"`
	UploadDir string `json:"upload_dir"`
}

type DBConfig struct {
	Path string `json:"path"`
}

type AppConfig struct {
	SessionTTL      Duration `json:"session_ttl"`
	SessionCleanup  Duration `json:"session_cleanup"`
	RateLimit       int      `json:"rate_limit"`
	RateWindow      Duration `json:"rate_window"`
	MaxMsgLength    int      `json:"max_msg_length"`
	HistoryLimit    int      `json:"history_limit"`
	HistoryMax      int      `json:"history_max"`
}

type Config struct {
	Server ServerConfig `json:"server"`
	Web    WebConfig    `json:"web"`
	DB     DBConfig     `json:"db"`
	App    AppConfig    `json:"app"`
}

// DefaultConfig returns the built-in defaults matching the original hardcoded values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			IP:           "127.0.0.1",
			Port:         8888,
			TLS:          false,
			IdleTimeout:  Duration(10 * time.Minute),
		},
		Web: WebConfig{
			Addr:      ":8080",
			UploadDir: "uploads",
		},
		DB: DBConfig{
			Path: "im.db",
		},
		App: AppConfig{
			SessionTTL:     Duration(24 * time.Hour),
			SessionCleanup: Duration(30 * time.Minute),
			RateLimit:      10,
			RateWindow:     Duration(1 * time.Minute),
			MaxMsgLength:   500,
			HistoryLimit:   300,
			HistoryMax:     500,
		},
	}
}

// Load loads configuration for the given environment.
// Load order: built-in defaults → config/{env}.json → environment variables (IM_*).
func Load(env string) (*Config, error) {
	cfg := DefaultConfig()

	// Load config file if it exists (non-fatal if missing).
	cfgFile := fmt.Sprintf("config/%s.json", env)
	if data, err := os.ReadFile(cfgFile); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", cfgFile, err)
		}
	}

	// Override with environment variables.
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides reads IM_* environment variables and overrides config fields.
func applyEnvOverrides(cfg *Config) {
	overrides := map[string]func(){
		"IM_SERVER_IP":      func() { cfg.Server.IP = getEnv("IM_SERVER_IP") },
		"IM_SERVER_PORT":    func() { cfg.Server.Port = getEnvInt("IM_SERVER_PORT") },
		"IM_SERVER_TLS":     func() { cfg.Server.TLS = getEnvBool("IM_SERVER_TLS") },
		"IM_IDLE_TIMEOUT":   func() { cfg.Server.IdleTimeout = parseEnvDuration("IM_IDLE_TIMEOUT") },
		"IM_WEB_ADDR":       func() { cfg.Web.Addr = getEnv("IM_WEB_ADDR") },
	"IM_UPLOAD_DIR":	func() { cfg.Web.UploadDir = getEnv("IM_UPLOAD_DIR") },
		"IM_DB_PATH":        func() { cfg.DB.Path = getEnv("IM_DB_PATH") },
		"IM_SESSION_TTL":    func() { cfg.App.SessionTTL = parseEnvDuration("IM_SESSION_TTL") },
		"IM_SESSION_CLEANUP": func() { cfg.App.SessionCleanup = parseEnvDuration("IM_SESSION_CLEANUP") },
		"IM_RATE_LIMIT":     func() { cfg.App.RateLimit = getEnvInt("IM_RATE_LIMIT") },
		"IM_RATE_WINDOW":    func() { cfg.App.RateWindow = parseEnvDuration("IM_RATE_WINDOW") },
		"IM_MAX_MSG_LENGTH": func() { cfg.App.MaxMsgLength = getEnvInt("IM_MAX_MSG_LENGTH") },
		"IM_HISTORY_LIMIT":  func() { cfg.App.HistoryLimit = getEnvInt("IM_HISTORY_LIMIT") },
		"IM_HISTORY_MAX":    func() { cfg.App.HistoryMax = getEnvInt("IM_HISTORY_MAX") },
	}

	for envName, apply := range overrides {
		if _, ok := lookupEnv(envName); ok {
			apply()
		}
	}
}

// --- env helpers ---

func getEnv(key string) string {
	v, _ := lookupEnv(key)
	return v
}

func getEnvInt(key string) int {
	v, ok := lookupEnv(key)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func getEnvBool(key string) bool {
	v, ok := lookupEnv(key)
	if !ok {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func parseEnvDuration(key string) Duration {
	v, ok := lookupEnv(key)
	if !ok {
		return Duration(0) // zero value signals "not set"
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return Duration(0)
	}
	return Duration(d)
}

// lookupEnv wraps os.LookupEnv, made replaceable for testing.
var lookupEnv = os.LookupEnv

// SetLookupEnv allows tests to inject a custom environment lookup.
func SetLookupEnv(fn func(string) (string, bool)) {
	lookupEnv = fn
}

// ResetLookupEnv restores the real os.LookupEnv.
func ResetLookupEnv() {
	lookupEnv = os.LookupEnv
}

// Ensure config struct fields are documented and complete.
var _ = reflect.TypeOf(Config{})
