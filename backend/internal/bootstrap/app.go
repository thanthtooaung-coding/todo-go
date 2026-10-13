// Package bootstrap wires the HTTP application and runs the server.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/config"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/database"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/middleware"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/handler"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/repository"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/routes"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/service"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/shared/responses"
)

// App is the wired HTTP application.
type App struct {
	Config config.Config
	Logger *slog.Logger
	DB     *gorm.DB
	Engine *gin.Engine
}

// New builds the router and todo dependencies.
func New(cfg config.Config, logger *slog.Logger, db *gorm.DB) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Env == config.EnvProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = false
	engine.RedirectFixedPath = false
	proxies := cfg.TrustedProxies
	if proxies == nil {
		proxies = []string{}
	}
	if err := engine.SetTrustedProxies(proxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}

	engine.Use(middleware.RequestID())
	engine.Use(middleware.AttachLogger(logger))
	engine.Use(middleware.AccessLog(logger))
	engine.Use(middleware.Recovery(logger))
	engine.Use(middleware.SecurityHeaders())
	engine.Use(middleware.LimitBody())
	engine.Use(middleware.Timeout(cfg.RequestTimeout))
	if len(cfg.AllowedOrigins) > 0 {
		engine.Use(middleware.CORS(cfg.AllowedOrigins))
	}
	if cfg.RateLimitRPS > 0 {
		engine.Use(middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst, len(cfg.TrustedProxies) > 0).Handler())
	}

	engine.GET("/healthz", func(c *gin.Context) {
		responses.Health(c, http.StatusOK, "ok")
	})
	engine.GET("/readyz", ready(db))
	engine.NoRoute(func(c *gin.Context) {
		responses.Error(c, http.StatusNotFound, string(apperrors.CodeNotFound), "resource not found", nil)
	})
	engine.NoMethod(func(c *gin.Context) {
		responses.Error(c, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
	})

	todoRepository := repository.New(db)
	todoService := service.New(todoRepository)
	todoHandler := handler.New(todoService)
	routes.Register(engine.Group("/api/v1"), todoHandler)

	return &App{Config: cfg, Logger: logger, DB: db, Engine: engine}, nil
}

func ready(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ping(ctx, db); err != nil {
			responses.Health(c, http.StatusServiceUnavailable, "unavailable")
			return
		}
		responses.Health(c, http.StatusOK, "ok")
	}
}

// Run checks the schema, serves HTTP, and shuts down when ctx is canceled.
func (a *App) Run(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, a.Config.Database.ConnectTimeout)
	defer cancel()
	if err := database.CheckSchema(checkCtx, a.DB); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              a.Config.HTTP.Address(),
		Handler:           a.Engine,
		ReadHeaderTimeout: a.Config.HTTP.ReadHeaderTimeout,
		ReadTimeout:       a.Config.HTTP.ReadTimeout,
		WriteTimeout:      a.Config.HTTP.WriteTimeout,
		IdleTimeout:       a.Config.HTTP.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(a.Logger.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() {
		a.Logger.Info("server starting", "addr", server.Addr)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.Config.HTTP.ShutdownTimeout)
		defer shutdownCancel()
		a.Logger.Info("server shutting down")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	}
}
