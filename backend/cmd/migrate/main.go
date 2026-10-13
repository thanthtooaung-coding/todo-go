// Command migrate applies versioned SQL migrations.
//
// Usage: migrate up|down|version
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/config"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "down" && args[0] != "version") {
		return fmt.Errorf("usage: migrate up|down|version")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	message, err := database.Apply(ctx, cfg.Database.URL(), cfg.MigrationsPath, args[0])
	if err != nil {
		return err
	}
	fmt.Println(message)
	return nil
}
