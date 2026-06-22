package db

import "embed"

//go:embed migrations/*.up.sql
var MigrationsFS embed.FS
