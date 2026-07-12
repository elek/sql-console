package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
)

// SQLClient is a generic DatabaseClient implementation built on top of the
// standard library database/sql package. It is shared by any driver that
// exposes a database/sql interface (PostgreSQL, TiDB/MySQL, ClickHouse, ...).
//
// Each backend supplies its own driver name, DSN, connection name (used as the
// interactive prompt) and a catalog query for ListTables, so this type needs no
// per-database knowledge.
type SQLClient struct {
	db            *sql.DB
	name          string
	listTablesSQL string
}

var _ DatabaseClient = (*SQLClient)(nil)

// NewSQLClient opens a database/sql connection using the given driver and DSN,
// pings it to fail fast on bad connections, and returns a ready SQLClient.
func NewSQLClient(ctx context.Context, driverName, dsn, name, listTablesSQL string) (*SQLClient, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s connection", driverName)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.Wrapf(err, "failed to connect to %s", driverName)
	}
	return &SQLClient{
		db:            db,
		name:          name,
		listTablesSQL: listTablesSQL,
	}, nil
}

// Execute runs a single query. Read-only statements are sent through
// QueryContext and their rows rendered; anything else goes through ExecContext
// and the affected-row count is printed.
func (c *SQLClient) Execute(ctx context.Context, query string) error {
	if isReadOnlyQuery([]string{query}) {
		return c.query(ctx, c.db, query)
	}
	return c.exec(ctx, c.db, query)
}

// ExecuteInTx runs all queries inside a single transaction, committing on
// success and rolling back on the first error.
func (c *SQLClient) ExecuteInTx(ctx context.Context, queries []string) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	for _, query := range queries {
		if query == "" {
			continue
		}
		if isReadOnlyQuery([]string{query}) {
			err = c.query(ctx, tx, query)
		} else {
			err = c.exec(ctx, tx, query)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return errors.Wrapf(err, "query failed and rollback failed: %v", rbErr)
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}
	return nil
}

// Close releases the underlying connection pool.
func (c *SQLClient) Close() {
	_ = c.db.Close()
}

// GetName returns the descriptive connection name (used as the prompt).
func (c *SQLClient) GetName() string {
	return c.name
}

// ListTables renders the backend-specific catalog query.
func (c *SQLClient) ListTables(ctx context.Context) error {
	return c.query(ctx, c.db, c.listTablesSQL)
}

// querier abstracts *sql.DB and *sql.Tx so query/exec work both standalone and
// inside a transaction.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// query runs a row-returning statement and renders it through a ResultWriter.
func (c *SQLClient) query(ctx context.Context, q querier, query string) error {
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return errors.WithStack(err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return errors.WithStack(err)
	}

	writer := GetResultWriter(outputFormat)
	writer.SetHeader(cols)

	for rows.Next() {
		// Scan into *interface{} targets so we get the driver's native values
		// and can detect NULL as nil.
		values := make([]interface{}, len(cols))
		scanTargets := make([]interface{}, len(cols))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return errors.WithStack(err)
		}

		row := make([]interface{}, len(cols))
		for i, v := range values {
			switch tv := v.(type) {
			case nil:
				row[i] = nil
			case []byte:
				// Many drivers return text/varchar as []byte; render as string.
				row[i] = string(tv)
			default:
				row[i] = tv
			}
		}
		writer.AppendRow(row)
	}
	if err := rows.Err(); err != nil {
		return errors.WithStack(err)
	}

	writer.Render()
	fmt.Println()
	return nil
}

// exec runs a non-row-returning statement and prints the affected-row count.
func (c *SQLClient) exec(ctx context.Context, q querier, query string) error {
	res, err := q.ExecContext(ctx, query)
	if err != nil {
		return errors.WithStack(err)
	}
	if affected, err := res.RowsAffected(); err == nil {
		fmt.Printf("%d row(s) affected\n", affected)
	}
	return nil
}
