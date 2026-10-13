// Package middleware contains HTTP middleware shared by every route.
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/logging"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/shared/responses"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
	maxBodyBytes    = 1 << 20
)

// RequestID accepts a safe client request ID or generates one.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if !validRequestID(id) {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// AttachLogger stores a request-scoped logger on the request context.
func AttachLogger(base *slog.Logger) gin.HandlerFunc {
	if base == nil {
		base = slog.Default()
	}
	return func(c *gin.Context) {
		logger := base
		if id := RequestIDFromGin(c); id != "" {
			logger = base.With("request_id", id)
		}
		c.Request = c.Request.WithContext(logging.WithContext(c.Request.Context(), logger))
		c.Next()
	}
}

// RequestIDFromGin returns the request ID stored for this request.
func RequestIDFromGin(c *gin.Context) string {
	value, ok := c.Get(requestIDKey)
	if !ok {
		return ""
	}
	id, _ := value.(string)
	return id
}

// AccessLog writes one structured line for each request except health checks.
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		if path == "/healthz" || path == "/readyz" {
			return
		}
		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFromGin(c),
			"client_ip", clientIP(c, false),
		}
		switch {
		case status >= 500:
			logger.Error("request", attrs...)
		case status >= 400:
			logger.Warn("request", attrs...)
		default:
			logger.Info("request", attrs...)
		}
	}
}

// Recovery converts a panic into a generic JSON error and logs the stack.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			logger.Error("panic recovered",
				"error", fmt.Sprint(rec),
				"request_id", RequestIDFromGin(c),
				"stack", string(debug.Stack()),
			)
			if !c.Writer.Written() {
				responses.Error(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
			}
			c.Abort()
		}()
		c.Next()
	}
}

// SecurityHeaders sets conservative response headers for a JSON API.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// LimitBody rejects request bodies larger than 1 MiB.
func LimitBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
		c.Next()
	}
}

// Timeout cancels the request context after the given duration.
func Timeout(limit time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), limit)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// CORS allows the configured browser origins. An origin of * allows any origin.
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, origin := range origins {
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || contains(allowed, origin)) {
			value := origin
			if allowAll {
				value = "*"
			}
			c.Header("Access-Control-Allow-Origin", value)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID, Location")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RateLimiter limits requests per client address.
type RateLimiter struct {
	mu         sync.Mutex
	visitors   map[string]*visitor
	limit      rate.Limit
	burst      int
	trustProxy bool
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter builds an in-memory per-client limiter.
func NewRateLimiter(rps float64, burst int, trustProxy bool) *RateLimiter {
	return &RateLimiter{
		visitors:   make(map[string]*visitor),
		limit:      rate.Limit(rps),
		burst:      burst,
		trustProxy: trustProxy,
	}
}

// Handler returns the Gin middleware.
func (l *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(clientIP(c, l.trustProxy)) {
			responses.Error(c, http.StatusTooManyRequests, "rate_limited", "too many requests", nil)
			return
		}
		c.Next()
	}
}

func (l *RateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.visitors) > 1024 {
		for key, item := range l.visitors {
			if now.Sub(item.lastSeen) > 10*time.Minute {
				delete(l.visitors, key)
			}
		}
	}
	item, ok := l.visitors[ip]
	if !ok {
		item = &visitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[ip] = item
	}
	item.lastSeen = now
	return item.limiter.Allow()
}

func clientIP(c *gin.Context, trustProxy bool) string {
	if trustProxy {
		return c.ClientIP()
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

func validRequestID(id string) bool {
	if len(id) < 8 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func contains(set map[string]struct{}, value string) bool {
	_, ok := set[value]
	return ok
}
