// Command server runs the TodoList HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/bootstrap"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/config"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/database"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	db, err := database.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			logger.Error("close database", "error", closeErr)
		}
	}()

	app, err := bootstrap.New(cfg, logger, db)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx)
}
