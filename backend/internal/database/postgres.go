// Package database opens PostgreSQL and applies versioned SQL migrations.
package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/config"
)

// Open connects to PostgreSQL, configures the pool, and verifies the connection.
// It does not change the schema.
func Open(cfg config.Database) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.URL()), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
		Logger: gormlogger.New(
			log.New(os.Stdout, "", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormlogger.Warn,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// Ping checks that the database accepts a query before the context ends.
func Ping(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

// Close releases the connection pool.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// CheckSchema reports whether the todos table exists.
// It does not create or alter tables.
func CheckSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`SELECT to_regclass('public.todos') IS NOT NULL`).Scan(&ready).Error
	if err != nil {
		return fmt.Errorf("check database schema: %w", err)
	}
	if !ready {
		return fmt.Errorf("database schema is not initialized; run migrations before starting the server")
	}
	return nil
}
