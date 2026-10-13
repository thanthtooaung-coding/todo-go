package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const schemaMigrationsSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

var migrationFilePattern = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)

type migration struct {
	Version  int64
	Name     string
	UpPath   string
	DownPath string
}

// Apply runs one migration command: up, down, or version.
func Apply(ctx context.Context, dsn, dir, command string) (string, error) {
	migrations, err := loadMigrationFiles(dir)
	if err != nil {
		return "", err
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", fmt.Errorf("connect database: %w", err)
	}
	defer conn.Close(context.Background())

	if err := execSQL(ctx, conn, schemaMigrationsSQL); err != nil {
		return "", fmt.Errorf("prepare schema_migrations: %w", err)
	}
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return "", err
	}

	switch command {
	case "up":
		pending, err := selectPending(migrations, applied)
		if err != nil {
			return "", err
		}
		if len(pending) == 0 {
			return "migrations are up to date", nil
		}
		var lines []string
		for _, item := range pending {
			script, err := os.ReadFile(item.UpPath)
			if err != nil {
				return "", fmt.Errorf("read migration %d: %w", item.Version, err)
			}
			wrapped := fmt.Sprintf("BEGIN;\n%s\nINSERT INTO schema_migrations (version) VALUES (%d);\nCOMMIT;", string(script), item.Version)
			if err := execSQL(ctx, conn, wrapped); err != nil {
				return "", fmt.Errorf("apply migration %d (%s): %w", item.Version, item.Name, err)
			}
			lines = append(lines, fmt.Sprintf("applied migration %d_%s", item.Version, item.Name))
		}
		return strings.Join(lines, "\n"), nil
	case "down":
		latest, ok, err := latestApplied(migrations, applied)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("no migrations to roll back")
		}
		script, err := os.ReadFile(latest.DownPath)
		if err != nil {
			return "", fmt.Errorf("read migration %d: %w", latest.Version, err)
		}
		wrapped := fmt.Sprintf("BEGIN;\n%s\nDELETE FROM schema_migrations WHERE version = %d;\nCOMMIT;", string(script), latest.Version)
		if err := execSQL(ctx, conn, wrapped); err != nil {
			return "", fmt.Errorf("roll back migration %d (%s): %w", latest.Version, latest.Name, err)
		}
		return fmt.Sprintf("rolled back migration %d_%s", latest.Version, latest.Name), nil
	case "version":
		if len(applied) == 0 {
			return "none", nil
		}
		sort.Slice(applied, func(i, j int) bool { return applied[i] < applied[j] })
		return strconv.FormatInt(applied[len(applied)-1], 10), nil
	default:
		return "", fmt.Errorf("unknown migration command %q", command)
	}
}

func loadMigrationFiles(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}
	byVersion := make(map[int64]*migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		match := migrationFilePattern.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf("migration file %s does not match NNN_name.up.sql", name)
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration file %s has an invalid version", name)
		}
		item := byVersion[version]
		if item == nil {
			item = &migration{Version: version, Name: match[2]}
			byVersion[version] = item
		}
		if item.Name != match[2] {
			return nil, fmt.Errorf("migration version %d has conflicting names", version)
		}
		path := filepath.Join(dir, name)
		switch match[3] {
		case "up":
			if item.UpPath != "" {
				return nil, fmt.Errorf("migration version %d has two up files", version)
			}
			item.UpPath = path
		case "down":
			if item.DownPath != "" {
				return nil, fmt.Errorf("migration version %d has two down files", version)
			}
			item.DownPath = path
		}
	}

	versions := make([]int64, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	out := make([]migration, 0, len(versions))
	for _, version := range versions {
		item := byVersion[version]
		if item.UpPath == "" || item.DownPath == "" {
			return nil, fmt.Errorf("migration version %d needs both up and down files", version)
		}
		out = append(out, *item)
	}
	return out, nil
}

func selectPending(all []migration, applied []int64) ([]migration, error) {
	appliedSet := make(map[int64]struct{}, len(applied))
	var maxApplied int64
	for _, version := range applied {
		appliedSet[version] = struct{}{}
		if version > maxApplied {
			maxApplied = version
		}
	}
	pending := make([]migration, 0)
	for _, item := range all {
		if _, ok := appliedSet[item.Version]; ok {
			continue
		}
		if maxApplied > 0 && item.Version < maxApplied {
			return nil, fmt.Errorf("migration %d is missing and a later migration is already applied", item.Version)
		}
		pending = append(pending, item)
	}
	return pending, nil
}

func latestApplied(all []migration, applied []int64) (migration, bool, error) {
	if len(applied) == 0 {
		return migration{}, false, nil
	}
	byVersion := make(map[int64]migration, len(all))
	for _, item := range all {
		byVersion[item.Version] = item
	}
	var latest int64
	for _, version := range applied {
		if version > latest {
			latest = version
		}
	}
	item, ok := byVersion[latest]
	if !ok {
		return migration{}, false, fmt.Errorf("applied migration %d has no file", latest)
	}
	return item, true, nil
}

func appliedVersions(ctx context.Context, conn *pgx.Conn) ([]int64, error) {
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("read schema_migrations: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	return versions, nil
}

func execSQL(ctx context.Context, conn *pgx.Conn, script string) error {
	results, err := conn.PgConn().Exec(ctx, script).ReadAll()
	if err != nil {
		return err
	}
	for _, result := range results {
		if result.Err != nil {
			return result.Err
		}
	}
	return nil
}
