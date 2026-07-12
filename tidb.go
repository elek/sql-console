package main

import (
	"context"
	"strings"

	// Register the go-sql-driver/mysql database/sql driver (driver name "mysql").
	_ "github.com/go-sql-driver/mysql"
)

// tidbListTablesSQL lists user tables across all non-system schemas.
const tidbListTablesSQL = `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('mysql','information_schema','performance_schema','sys','metrics_schema') ORDER BY table_schema, table_name`

// NewTiDBClient connects to TiDB using the MySQL driver and returns a generic
// SQLClient configured with the TiDB/MySQL catalog query.
func NewTiDBClient(ctx context.Context, dsn string) (DatabaseClient, error) {
	return NewSQLClient(ctx, "mysql", dsn, tidbName(dsn), tidbListTablesSQL)
}

// tidbName derives a readable prompt name from the DSN. For a MySQL DSN of the
// form user:pass@tcp(host:port)/dbname it uses the database name (after the
// last '/'); otherwise it falls back to the raw DSN.
func tidbName(dsn string) string {
	if idx := strings.LastIndex(dsn, "/"); idx != -1 {
		if db := dsn[idx+1:]; db != "" {
			// Strip any DSN query parameters (e.g. ?parseTime=true).
			if q := strings.IndexByte(db, '?'); q != -1 {
				db = db[:q]
			}
			if db != "" {
				return db
			}
		}
	}
	return dsn
}
