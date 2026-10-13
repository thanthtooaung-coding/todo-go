//go:build integration

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/database"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
)

func TestRepositoryAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("TODO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TODO_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "..", "migrations")
	if _, err := database.Apply(ctx, dsn, migrations, "up"); err != nil {
		t.Fatal(err)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`TRUNCATE todos`).Error; err != nil {
		t.Fatal(err)
	}

	repo := New(db)
	due := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	first := &models.Todo{Title: "100% milk", Description: "dairy", DueDate: &due}
	second := &models.Todo{Title: "Read book", Completed: true}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(ctx, first.ID)
	if err != nil || got.Title != "100% milk" || got.Completed {
		t.Fatalf("%+v %v", got, err)
	}

	completed := false
	page, err := repo.List(ctx, ListParams{
		Page:      1,
		PageSize:  20,
		Completed: &completed,
		Query:     "%",
		Sort:      models.SortTitle,
		Order:     models.OrderAsc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != first.ID {
		t.Fatalf("search escaped incorrectly: %+v", page)
	}

	first.DueDate = nil
	first.Title = "Buy milk"
	if err := repo.Update(ctx, first); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, first.ID)
	if err != nil || got.DueDate != nil || got.Title != "Buy milk" {
		t.Fatalf("%+v %v", got, err)
	}

	if err := repo.Delete(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, second.ID); err == nil {
		t.Fatal("expected deleted todo to be missing")
	}
}
