package migrations

import "embed"

// MigrationFS embeds all .sql migration files.
//
//go:embed *.sql
var MigrationFS embed.FS
