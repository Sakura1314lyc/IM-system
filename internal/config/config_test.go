package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.IP != "127.0.0.1" {
		t.Errorf("expected default ip '127.0.0.1', got %q", cfg.Server.IP)
	}
	if cfg.Server.Port != 8888 {
		t.Errorf("expected default port 8888, got %d", cfg.Server.Port)
	}
	if cfg.Server.IdleTimeout.ToDuration() != 10*time.Minute {
		t.Errorf("expected idle timeout 10m, got %v", cfg.Server.IdleTimeout.ToDuration())
	}
	if cfg.App.SessionTTL.ToDuration() != 24*time.Hour {
		t.Errorf("expected session TTL 24h, got %v", cfg.App.SessionTTL.ToDuration())
	}
	if cfg.App.RateLimit != 10 {
		t.Errorf("expected rate limit 10, got %d", cfg.App.RateLimit)
	}
	if cfg.App.MaxMsgLength != 500 {
		t.Errorf("expected max msg length 500, got %d", cfg.App.MaxMsgLength)
	}
}

func TestDurationJSON(t *testing.T) {
	t.Run("unmarshal valid duration", func(t *testing.T) {
		var d Duration
		data := []byte(`"5m"`)
		if err := d.UnmarshalJSON(data); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.ToDuration() != 5*time.Minute {
			t.Errorf("expected 5m, got %v", d.ToDuration())
		}
	})

	t.Run("unmarshal invalid duration", func(t *testing.T) {
		var d Duration
		data := []byte(`"not-a-duration"`)
		if err := d.UnmarshalJSON(data); err == nil {
			t.Error("expected error for invalid duration")
		}
	})

	t.Run("marshal and unmarshal round trip", func(t *testing.T) {
		original := Duration(3 * time.Hour)
		data, err := original.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var restored Duration
		if err := restored.UnmarshalJSON(data); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if original != restored {
			t.Errorf("round trip: got %v, want %v", restored, original)
		}
	})
}

func TestLoadDefaultConfig(t *testing.T) {
	// Load with non-existent env — should return defaults without error
	cfg, err := Load("nonexistent-env-12345")
	if err != nil {
		t.Fatalf("expected no error for non-existent env, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Server.Port != 8888 {
		t.Errorf("expected default port 8888, got %d", cfg.Server.Port)
	}
}

func TestEnvOverride(t *testing.T) {
	// Save original lookup and restore after test
	original := lookupEnv
	defer func() { lookupEnv = original }()

	SetLookupEnv(func(key string) (string, bool) {
		switch key {
		case "IM_SERVER_PORT":
			return "9999", true
		case "IM_SESSION_TTL":
			return "1h", true
		case "IM_MAX_MSG_LENGTH":
			return "100", true
		case "IM_SERVER_TLS":
			return "true", true
		default:
			return "", false
		}
	})

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999 from env, got %d", cfg.Server.Port)
	}
	if cfg.App.SessionTTL.ToDuration() != time.Hour {
		t.Errorf("expected session TTL 1h from env, got %v", cfg.App.SessionTTL.ToDuration())
	}
	if cfg.App.MaxMsgLength != 100 {
		t.Errorf("expected max msg length 100 from env, got %d", cfg.App.MaxMsgLength)
	}
	if !cfg.Server.TLS {
		t.Error("expected TLS true from env")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp directory with config/ subdirectory (matches Load() path logic)
	tmpDir := t.TempDir()
	if err := os.Mkdir(tmpDir+"/config", 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configFile := tmpDir + "/config/test_config.json"
	content := `{
		"server": { "ip": "10.0.0.1", "port": 1234, "tls": true, "idle_timeout": "30s" },
		"web": { "addr": ":9090" },
		"db": { "path": "/tmp/test.db" },
		"app": {
			"session_ttl": "12h",
			"session_cleanup": "15m",
			"rate_limit": 50,
			"rate_window": "30s",
			"max_msg_length": 1000,
			"history_limit": 200,
			"history_max": 400
		}
	}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change working directory so Load() finds config/test_config.json
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	cfg, err := Load("test_config")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Server.IP != "10.0.0.1" {
		t.Errorf("expected ip '10.0.0.1', got %q", cfg.Server.IP)
	}
	if cfg.Server.Port != 1234 {
		t.Errorf("expected port 1234, got %d", cfg.Server.Port)
	}
	if cfg.Web.Addr != ":9090" {
		t.Errorf("expected web addr ':9090', got %q", cfg.Web.Addr)
	}
	if cfg.DB.Path != "/tmp/test.db" {
		t.Errorf("expected db path '/tmp/test.db', got %q", cfg.DB.Path)
	}
	if cfg.App.RateLimit != 50 {
		t.Errorf("expected rate limit 50, got %d", cfg.App.RateLimit)
	}
	if cfg.App.MaxMsgLength != 1000 {
		t.Errorf("expected max msg length 1000, got %d", cfg.App.MaxMsgLength)
	}
	if !cfg.Server.TLS {
		t.Error("expected TLS true")
	}
}
