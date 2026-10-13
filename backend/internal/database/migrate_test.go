package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectPending(t *testing.T) {
	all := []migration{{Version: 1, Name: "create_todos"}, {Version: 2, Name: "add_index"}}
	pending, err := selectPending(all, nil)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending %v err %v", pending, err)
	}
	pending, err = selectPending(all, []int64{1})
	if err != nil || len(pending) != 1 || pending[0].Version != 2 {
		t.Fatalf("pending %v err %v", pending, err)
	}
	if _, err := selectPending(all, []int64{2}); err == nil {
		t.Fatal("expected out-of-order migration error")
	}
}

func TestLoadMigrationFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_create_todos.up.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000001_create_todos.down.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	migrations, err := loadMigrationFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 1 || migrations[0].Version != 1 || migrations[0].Name != "create_todos" {
		t.Fatalf("%+v", migrations)
	}

	if err := os.WriteFile(filepath.Join(dir, "notes.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMigrationFiles(dir); err == nil {
		t.Fatal("expected invalid file name error")
	}
}
