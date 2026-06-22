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

// MigrateUp reads all *.up.sql files in dir and executes them in lexical order.
func MigrateUp(ctx context.Context, conn *pgx.Conn, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files)

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec migration %s: %w", filepath.Base(f), err)
		}
		fmt.Printf("applied migration: %s\n", filepath.Base(f))
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

	for _, f := range files {
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
			return fmt.Errorf("exec migration %s: %w", filepath.Base(f), err)
		}
		fmt.Printf("applied migration: %s\n", filepath.Base(f))
	}
	return nil
}

// MigrationDir returns the embedded migrations directory.
func MigrationDir() string {
	return filepath.Join("internal", "db", "migrations")
}

