package db

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
)

const migrationsTable = "schema_migrations"

// MigrateUp reads all *.up.sql files in dir and executes them in lexical order.
func MigrateUp(ctx context.Context, conn *pgx.Conn, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files)

	if err := ensureMigrationsTable(ctx, conn); err != nil {
		return err
	}

	applied, err := loadAppliedVersions(ctx, conn)
	if err != nil {
		return err
	}

	for _, f := range files {
		version := filepath.Base(f)
		if applied[version] {
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec migration %s: %w", version, err)
		}
		if err := recordMigration(ctx, conn, version); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		fmt.Printf("applied migration: %s\n", version)
	}
	return nil
}

// MigrateUpFS reads all *.up.sql files from the provided filesystem and executes them.
func MigrateUpFS(ctx context.Context, conn *pgx.Conn, fsys fs.FS) error {
	files, err := fs.Glob(fsys, "migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files)

	if err := ensureMigrationsTable(ctx, conn); err != nil {
		return err
	}

	applied, err := loadAppliedVersions(ctx, conn)
	if err != nil {
		return err
	}

	for _, f := range files {
		version := filepath.Base(f)
		if applied[version] {
			continue
		}
		r, err := fsys.Open(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		sql, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec migration %s: %w", version, err)
		}
		if err := recordMigration(ctx, conn, version); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		fmt.Printf("applied migration: %s\n", version)
	}
	return nil
}

func ensureMigrationsTable(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT now()
		)
	`, migrationsTable))
	return err
}

func loadAppliedVersions(ctx context.Context, conn *pgx.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT version FROM %s", migrationsTable))
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		versions[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration versions: %w", err)
	}
	return versions, nil
}

func recordMigration(ctx context.Context, conn *pgx.Conn, version string) error {
	_, err := conn.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s (version) VALUES ($1) ON CONFLICT (version) DO NOTHING", migrationsTable),
		version,
	)
	return err
}

// MigrationDir returns the embedded migrations directory.
func MigrationDir() string {
	return filepath.Join("internal", "db", "migrations")
}
