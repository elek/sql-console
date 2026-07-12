package main

import (
	"context"
	"net/url"
	"strings"

	// Register the clickhouse-go database/sql driver (driver name "clickhouse").
	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// clickhouseListTablesSQL lists user tables across all non-system databases.
const clickhouseListTablesSQL = `SELECT database, name FROM system.tables WHERE database NOT IN ('system','INFORMATION_SCHEMA','information_schema') ORDER BY database, name`

// NewClickHouseClient connects to ClickHouse using the clickhouse-go driver and
// returns a generic SQLClient configured with the ClickHouse catalog query.
func NewClickHouseClient(ctx context.Context, dsn string) (DatabaseClient, error) {
	return NewSQLClient(ctx, "clickhouse", dsn, clickhouseName(dsn), clickhouseListTablesSQL)
}

// clickhouseName derives a readable prompt name from the DSN. For URL-style
// DSNs it uses the database name (the URL path); otherwise it falls back to the
// raw DSN.
func clickhouseName(dsn string) string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		if db := strings.TrimPrefix(u.Path, "/"); db != "" {
			return db
		}
	}
	return dsn
}
