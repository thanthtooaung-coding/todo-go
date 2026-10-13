package config

import (
	"strings"
	"testing"
	"time"
)

func TestFromLookupUsesDefaults(t *testing.T) {
	cfg, err := fromLookup(func(key string) (string, bool) {
		switch key {
		case "DB_HOST":
			return "localhost", true
		case "DB_USER":
			return "todo", true
		case "DB_PASSWORD":
			return "secret", true
		case "DB_NAME":
			return "todo", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != EnvDevelopment || cfg.HTTP.Port != 8080 || cfg.Database.Port != 5432 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Database.Password != "secret" {
		t.Fatal("password was not kept")
	}
	if !strings.Contains(cfg.Database.URL(), "localhost") || strings.Contains(cfg.Database.URL(), "secret") == false {
		t.Fatal(cfg.Database.URL())
	}
}

func TestFromLookupDatabaseURLPreservesParameters(t *testing.T) {
	cfg, err := fromLookup(func(key string) (string, bool) {
		if key == "DATABASE_URL" {
			return "postgresql://app:secret@db.example/neondb?sslmode=require&channel_binding=require", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Database.URL()
	if cfg.Database.Host != "db.example" || cfg.Database.Name != "neondb" || cfg.Database.SSLMode != "require" {
		t.Fatalf("parsed database host %s name %s sslmode %s", cfg.Database.Host, cfg.Database.Name, cfg.Database.SSLMode)
	}
	if !strings.Contains(got, "channel_binding=require") || !strings.Contains(got, "sslmode=require") || !strings.Contains(got, "connect_timeout=5") {
		t.Fatal(got)
	}
}

func TestFromLookupRequiresDatabase(t *testing.T) {
	_, err := fromLookup(func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "DB_HOST") {
		t.Fatalf("expected DB_HOST error, got %v", err)
	}
}

func TestFromLookupRejectsInvalidValues(t *testing.T) {
	base := func(extra string) func(string) (string, bool) {
		values := map[string]string{
			"DB_HOST":     "localhost",
			"DB_USER":     "todo",
			"DB_PASSWORD": "todo",
			"DB_NAME":     "todo",
		}
		key, value, _ := strings.Cut(extra, "=")
		values[key] = value
		return func(k string) (string, bool) {
			v, ok := values[k]
			return v, ok
		}
	}
	cases := []string{
		"HTTP_PORT=70000",
		"APP_ENV=staging",
		"DB_SSLMODE=prefer",
		"ALLOWED_ORIGINS=not a url",
		"TRUSTED_PROXIES=not-an-ip",
		"RATE_LIMIT_RPS=-1",
		"REQUEST_TIMEOUT=soon",
	}
	for _, item := range cases {
		t.Run(item, func(t *testing.T) {
			if _, err := fromLookup(base(item)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestFromLookupParsesDurationsAndOrigins(t *testing.T) {
	cfg, err := fromLookup(func(key string) (string, bool) {
		values := map[string]string{
			"DB_HOST":         "db.internal",
			"DB_USER":         "todo",
			"DB_PASSWORD":     "p@ss word",
			"DB_NAME":         "todo",
			"REQUEST_TIMEOUT": "3s",
			"ALLOWED_ORIGINS": "https://app.example",
			"RATE_LIMIT_RPS":  "0",
		}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RequestTimeout != 3*time.Second {
		t.Fatal(cfg.RequestTimeout)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.RateLimitRPS != 0 {
		t.Fatalf("%+v", cfg)
	}
	if !strings.Contains(cfg.Database.URL(), "p%40ss+word") && !strings.Contains(cfg.Database.URL(), "p%40ss%20word") {
		t.Fatal(cfg.Database.URL())
	}
}
