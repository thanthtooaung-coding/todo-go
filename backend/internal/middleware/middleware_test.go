package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestRequestIDAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.Use(NewRateLimiter(1, 1, false).Handler())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	first := httptest.NewRequest(http.MethodGet, "/ping", nil)
	first.Header.Set("X-Request-ID", "abc-12345")
	first.RemoteAddr = "192.0.2.1:1000"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, first)
	if rec.Code != http.StatusNoContent {
		t.Fatal(rec.Code)
	}
	if rec.Header().Get("X-Request-ID") != "abc-12345" {
		t.Fatal(rec.Header().Get("X-Request-ID"))
	}

	second := httptest.NewRequest(http.MethodGet, "/ping", nil)
	second.Header.Set("X-Request-ID", "bad id")
	second.RemoteAddr = "192.0.2.1:1000"
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") == "bad id" || rec.Header().Get("X-Request-ID") == "" {
		t.Fatal(rec.Header().Get("X-Request-ID"))
	}
}
