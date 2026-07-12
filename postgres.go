package main

import (
	"context"
	"net/url"
	"strings"

	// Register the pgx database/sql stdlib driver (driver name "pgx").
	_ "github.com/jackc/pgx/v5/stdlib"
)

// postgresListTablesSQL lists user tables across all non-system schemas.
const postgresListTablesSQL = `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema, table_name`

// NewPostgresClient connects to PostgreSQL using the pgx stdlib driver and
// returns a generic SQLClient configured with the Postgres catalog query.
func NewPostgresClient(ctx context.Context, dsn string) (DatabaseClient, error) {
	return NewSQLClient(ctx, "pgx", dsn, postgresName(dsn), postgresListTablesSQL)
}

// postgresName derives a readable prompt name from the DSN. For URL-style DSNs
// it uses the database name (the URL path); otherwise it falls back to the DSN.
func postgresName(dsn string) string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		if db := strings.TrimPrefix(u.Path, "/"); db != "" {
			return db
		}
	}
	return dsn
}
