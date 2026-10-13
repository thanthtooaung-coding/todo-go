package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/config"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestHealthAndUnknownRoute(t *testing.T) {
	app, err := New(config.Config{
		Env:            config.EnvTest,
		RequestTimeout: 1,
		RateLimitRPS:   0,
	}, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		t.Fatal(err)
	}

	health := httptest.NewRecorder()
	app.Engine.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health %d %s", health.Code, health.Body.String())
	}

	ready := httptest.NewRecorder()
	app.Engine.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready %d %s", ready.Code, ready.Body.String())
	}

	missing := httptest.NewRecorder()
	app.Engine.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing %d %s", missing.Code, missing.Body.String())
	}
}
