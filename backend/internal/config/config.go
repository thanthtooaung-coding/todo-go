// Package config loads and validates process configuration.
package config

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// EnvDevelopment is the default local environment.
	EnvDevelopment = "development"
	// EnvTest is for automated tests.
	EnvTest = "test"
	// EnvProduction enables release HTTP mode.
	EnvProduction = "production"
)

// Config is the validated application configuration.
type Config struct {
	Env            string
	LogLevel       string
	HTTP           HTTP
	Database       Database
	RequestTimeout time.Duration
	AllowedOrigins []string
	TrustedProxies []string
	RateLimitRPS   float64
	RateLimitBurst int
	MigrationsPath string
}

// HTTP holds the public server timeouts and bind address.
type HTTP struct {
	Host              string
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Address returns the host:port bind address.
func (h HTTP) Address() string {
	return net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
}

// Database holds PostgreSQL connection settings.
type Database struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	rawURL          string
}

// URL returns the PostgreSQL connection URL.
// A configured DATABASE_URL is preserved, including its query parameters.
func (d Database) URL() string {
	if d.rawURL != "" {
		return d.rawURL
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   "/" + d.Name,
	}
	query := u.Query()
	query.Set("sslmode", d.SSLMode)
	seconds := int(d.ConnectTimeout.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	query.Set("connect_timeout", strconv.Itoa(seconds))
	u.RawQuery = query.Encode()
	return u.String()
}

func (d *Database) applyURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a postgres URL")
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return fmt.Errorf("DATABASE_URL must include a user")
	}
	password, ok := parsed.User.Password()
	if !ok || password == "" {
		return fmt.Errorf("DATABASE_URL must include a password")
	}
	name := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("DATABASE_URL must include a database name")
	}
	port := 5432
	if parsed.Port() != "" {
		parsedPort, err := strconv.Atoi(parsed.Port())
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return fmt.Errorf("DATABASE_URL has an invalid port")
		}
		port = parsedPort
	}
	query := parsed.Query()
	sslMode := query.Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
		query.Set("sslmode", sslMode)
	}
	if err := validateSSLMode(sslMode); err != nil {
		return fmt.Errorf("DATABASE_URL sslmode must be disable, require, verify-ca, or verify-full")
	}
	if query.Get("connect_timeout") == "" {
		seconds := int(d.ConnectTimeout.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		query.Set("connect_timeout", strconv.Itoa(seconds))
	}
	parsed.RawQuery = query.Encode()
	d.Host = parsed.Hostname()
	d.Port = port
	d.User = parsed.User.Username()
	d.Password = password
	d.Name = name
	d.SSLMode = sslMode
	d.rawURL = parsed.String()
	return nil
}

func validateSSLMode(mode string) error {
	switch mode {
	case "disable", "require", "verify-ca", "verify-full":
		return nil
	default:
		return fmt.Errorf("DB_SSLMODE must be disable, require, verify-ca, or verify-full")
	}
}

// Load reads an optional .env file and then the process environment.
// Existing environment variables win over .env values.
func Load() (Config, error) {
	fileValues, err := readDotEnv(".env")
	if err != nil {
		return Config{}, err
	}
	return fromLookup(func(key string) (string, bool) {
		if value, ok := os.LookupEnv(key); ok {
			return value, true
		}
		value, ok := fileValues[key]
		return value, ok
	})
}

func fromLookup(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{
		Env:            "development",
		LogLevel:       "info",
		MigrationsPath: "migrations",
		RequestTimeout: 10 * time.Second,
		HTTP: HTTP{
			Host:              "0.0.0.0",
			Port:              8080,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownTimeout:   15 * time.Second,
		},
		Database: Database{
			Port:            5432,
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			ConnectTimeout:  5 * time.Second,
		},
		RateLimitRPS:   20,
		RateLimitBurst: 40,
	}

	if err := setString(&cfg.Env, lookup, "APP_ENV", false); err != nil {
		return Config{}, err
	}
	switch cfg.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}

	if err := setString(&cfg.LogLevel, lookup, "LOG_LEVEL", false); err != nil {
		return Config{}, err
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}

	if err := setString(&cfg.HTTP.Host, lookup, "HTTP_HOST", false); err != nil {
		return Config{}, err
	}
	if err := setInt(&cfg.HTTP.Port, lookup, "HTTP_PORT", false); err != nil {
		return Config{}, err
	}
	if cfg.HTTP.Port < 1 || cfg.HTTP.Port > 65535 {
		return Config{}, fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}
	if err := setDuration(&cfg.HTTP.ReadHeaderTimeout, lookup, "READ_HEADER_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.HTTP.ReadTimeout, lookup, "READ_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.HTTP.WriteTimeout, lookup, "WRITE_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.HTTP.IdleTimeout, lookup, "IDLE_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.HTTP.ShutdownTimeout, lookup, "SHUTDOWN_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.RequestTimeout, lookup, "REQUEST_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if cfg.RequestTimeout <= 0 || cfg.HTTP.ReadHeaderTimeout <= 0 || cfg.HTTP.ReadTimeout <= 0 || cfg.HTTP.WriteTimeout <= 0 || cfg.HTTP.IdleTimeout <= 0 || cfg.HTTP.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP timeouts must be greater than zero")
	}

	databaseURL, hasDatabaseURL := lookup("DATABASE_URL")
	if hasDatabaseURL {
		databaseURL = strings.TrimSpace(databaseURL)
	}
	if databaseURL == "" {
		if err := setString(&cfg.Database.Host, lookup, "DB_HOST", true); err != nil {
			return Config{}, err
		}
		if err := setInt(&cfg.Database.Port, lookup, "DB_PORT", false); err != nil {
			return Config{}, err
		}
		if cfg.Database.Port < 1 || cfg.Database.Port > 65535 {
			return Config{}, fmt.Errorf("DB_PORT must be between 1 and 65535")
		}
		if err := setString(&cfg.Database.User, lookup, "DB_USER", true); err != nil {
			return Config{}, err
		}
		if err := setString(&cfg.Database.Password, lookup, "DB_PASSWORD", true); err != nil {
			return Config{}, err
		}
		if err := setString(&cfg.Database.Name, lookup, "DB_NAME", true); err != nil {
			return Config{}, err
		}
		if err := setString(&cfg.Database.SSLMode, lookup, "DB_SSLMODE", false); err != nil {
			return Config{}, err
		}
		if err := validateSSLMode(cfg.Database.SSLMode); err != nil {
			return Config{}, err
		}
	}
	if err := setInt(&cfg.Database.MaxOpenConns, lookup, "DB_MAX_OPEN_CONNS", false); err != nil {
		return Config{}, err
	}
	if err := setInt(&cfg.Database.MaxIdleConns, lookup, "DB_MAX_IDLE_CONNS", false); err != nil {
		return Config{}, err
	}
	if cfg.Database.MaxOpenConns < 1 || cfg.Database.MaxIdleConns < 0 || cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
		return Config{}, fmt.Errorf("database pool settings are invalid")
	}
	if err := setDuration(&cfg.Database.ConnMaxLifetime, lookup, "DB_CONN_MAX_LIFETIME", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.Database.ConnMaxIdleTime, lookup, "DB_CONN_MAX_IDLE_TIME", false); err != nil {
		return Config{}, err
	}
	if err := setDuration(&cfg.Database.ConnectTimeout, lookup, "DB_CONNECT_TIMEOUT", false); err != nil {
		return Config{}, err
	}
	if cfg.Database.ConnMaxLifetime <= 0 || cfg.Database.ConnMaxIdleTime <= 0 || cfg.Database.ConnectTimeout <= 0 {
		return Config{}, fmt.Errorf("database timeouts must be greater than zero")
	}
	if databaseURL != "" {
		if err := cfg.Database.applyURL(databaseURL); err != nil {
			return Config{}, err
		}
	}

	origins, err := csv(lookup, "ALLOWED_ORIGINS")
	if err != nil {
		return Config{}, err
	}
	for _, origin := range origins {
		if origin == "*" {
			continue
		}
		parsed, parseErr := url.Parse(origin)
		if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" {
			return Config{}, fmt.Errorf("ALLOWED_ORIGINS contains an invalid origin %q", origin)
		}
	}
	cfg.AllowedOrigins = origins

	proxies, err := csv(lookup, "TRUSTED_PROXIES")
	if err != nil {
		return Config{}, err
	}
	for _, proxy := range proxies {
		if ip := net.ParseIP(proxy); ip == nil {
			if _, _, parseErr := net.ParseCIDR(proxy); parseErr != nil {
				return Config{}, fmt.Errorf("TRUSTED_PROXIES contains an invalid proxy %q", proxy)
			}
		}
	}
	cfg.TrustedProxies = proxies

	if err := setFloat(&cfg.RateLimitRPS, lookup, "RATE_LIMIT_RPS", false); err != nil {
		return Config{}, err
	}
	if err := setInt(&cfg.RateLimitBurst, lookup, "RATE_LIMIT_BURST", false); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitRPS < 0 || (cfg.RateLimitRPS > 0 && cfg.RateLimitBurst < 1) {
		return Config{}, fmt.Errorf("rate limit settings are invalid")
	}

	if err := setString(&cfg.MigrationsPath, lookup, "MIGRATIONS_PATH", false); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.MigrationsPath) == "" {
		return Config{}, fmt.Errorf("MIGRATIONS_PATH is required")
	}

	return cfg, nil
}

func setString(dest *string, lookup func(string) (string, bool), key string, required bool) error {
	value, ok := lookup(key)
	if !ok {
		if required {
			return fmt.Errorf("%s is required", key)
		}
		return nil
	}
	if key != "DB_PASSWORD" {
		value = strings.TrimSpace(value)
	}
	if required && value == "" {
		return fmt.Errorf("%s is required", key)
	}
	*dest = value
	return nil
}

func setInt(dest *int, lookup func(string) (string, bool), key string, required bool) error {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		if required {
			return fmt.Errorf("%s is required", key)
		}
		return nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("%s must be an integer", key)
	}
	*dest = parsed
	return nil
}

func setFloat(dest *float64, lookup func(string) (string, bool), key string, required bool) error {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		if required {
			return fmt.Errorf("%s is required", key)
		}
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return fmt.Errorf("%s must be a number", key)
	}
	*dest = parsed
	return nil
}

func setDuration(dest *time.Duration, lookup func(string) (string, bool), key string, required bool) error {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		if required {
			return fmt.Errorf("%s is required", key)
		}
		return nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("%s must be a duration such as 10s or 1m", key)
	}
	*dest = parsed
	return nil
}

func csv(lookup func(string) (string, bool), key string) ([]string, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("%s contains an empty value", key)
		}
		out = append(out, part)
	}
	return out, nil
}

func readDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s line %d is invalid", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("%s line %d has an invalid key", path, lineNo)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return values, nil
}
