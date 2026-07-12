package main

import (
	"context"
	"encoding/hex"
	"strings"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/pkg/errors"

	"fmt"
	"time"

	"google.golang.org/api/iterator"
)

type SpannerClient struct {
	client            *spanner.Client
	name              string
	transaction       *spanner.ReadWriteTransaction
	staleness         time.Duration
	exactTimestamp    time.Time
	useExactTimestamp bool
}

func (s *SpannerClient) ExecuteInTx(ctx context.Context, queries []string) error {
	return Execute(ctx, s.client, queries, s.staleness, s.exactTimestamp, s.useExactTimestamp)
}

var _ DatabaseClient = (*SpannerClient)(nil)

func NewSpannerClient(ctx context.Context, connectionString string, prompt string, staleness time.Duration, exactTimestamp time.Time, useExactTimestamp bool) (*SpannerClient, error) {
	client, err := spanner.NewClientWithConfig(ctx, connectionString, spanner.ClientConfig{
		SessionPoolConfig:    spanner.DefaultSessionPoolConfig,
		SessionLabels:        map[string]string{"application_name": "sql-console"},
		DisableRouteToLeader: false,
	})
	if err != nil {
		return nil, err
	}

	return &SpannerClient{
		client:            client,
		name:              prompt,
		staleness:         staleness,
		exactTimestamp:    exactTimestamp,
		useExactTimestamp: useExactTimestamp,
	}, nil
}

func (s *SpannerClient) Execute(ctx context.Context, query string) error {
	return Execute(ctx, s.client, []string{query}, s.staleness, s.exactTimestamp, s.useExactTimestamp)
}

func (s *SpannerClient) Close() {
	s.client.Close()
}

func (s *SpannerClient) GetName() string {
	return s.name
}

// isReadOnlyQuery checks if all queries are read-only
func isReadOnlyQuery(queries []string) bool {
	for _, q := range queries {
		q = strings.ToUpper(strings.TrimSpace(q))
		q = strings.TrimSpace(removeComments(q))
		if strings.HasPrefix(q, "INSERT") || strings.HasPrefix(q, "UPDATE") ||
			strings.HasPrefix(q, "DELETE") || strings.HasPrefix(q, "CREATE") ||
			strings.HasPrefix(q, "DROP") || strings.HasPrefix(q, "ALTER") {
			return false
		}
	}
	return true
}

func removeComments(q string) string {
	var result []string
	for _, line := range strings.Split(q, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--") {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func (s *SpannerClient) ListTables(ctx context.Context) error {
	writer := GetResultWriter(outputFormat)

	// Set up header
	writer.SetHeader([]string{"Table Name"})

	// Query for all tables in the database
	stmt := spanner.Statement{
		SQL: `SELECT table_name 
		      FROM information_schema.tables 
		      WHERE table_catalog = '' AND table_schema = '' 
		      ORDER BY table_name`,
	}

	// Use stale reads if staleness is set
	singleUse := s.client.Single()
	if s.useExactTimestamp {
		singleUse = singleUse.WithTimestampBound(spanner.ReadTimestamp(s.exactTimestamp))
	} else if s.staleness > 0 {
		singleUse = singleUse.WithTimestampBound(spanner.ExactStaleness(s.staleness))
	}
	iter := singleUse.Query(ctx, stmt)
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return err
		}

		var tableName string
		if err := row.Columns(&tableName); err != nil {
			return err
		}

		writer.AppendRow([]interface{}{tableName})
	}

	writer.Render()
	fmt.Println()
	return nil
}

func Execute(ctx context.Context, client *spanner.Client, queries []string, staleness time.Duration, exactTimestamp time.Time, useExactTimestamp bool) error {
	writer := GetResultWriter(outputFormat)

	var headerPrinted bool

	// If we have staleness set and only read queries, use stale reads
	if isReadOnlyQuery(queries) {
		// Create a read-only transaction with the staleness bound
		ro := client.ReadOnlyTransaction()
		if useExactTimestamp {
			ro = ro.WithTimestampBound(spanner.ReadTimestamp(exactTimestamp))
		} else if staleness > 0 {
			ro = ro.WithTimestampBound(spanner.ExactStaleness(staleness))
		}
		defer ro.Close()

		for _, query := range queries {
			if query == "" {
				continue
			}
			err := ro.Query(ctx, spanner.Statement{
				SQL: query,
			}).Do(func(r *spanner.Row) error {
				if !headerPrinted {
					var header []string
					for _, name := range r.ColumnNames() {
						header = append(header, name)
					}
					writer.SetHeader(header)
					headerPrinted = true
				}
				writer.AppendRow(convertToRow(r))
				return nil
			})
			if err != nil {
				return errors.WithStack(err)
			}
		}

		writer.Render()
		fmt.Println()
		return nil
	}

	// For write transactions or no staleness, use read-write transaction
	_, err := client.ReadWriteTransaction(ctx, func(ctx context.Context, transaction *spanner.ReadWriteTransaction) error {
		for _, query := range queries {
			if query == "" {
				continue
			}
			err := transaction.Query(ctx, spanner.Statement{
				SQL: query,
			}).Do(func(r *spanner.Row) error {
				if !headerPrinted {
					var header []string
					for _, name := range r.ColumnNames() {
						header = append(header, name)
					}
					writer.SetHeader(header)
					headerPrinted = true
				}
				writer.AppendRow(convertToRow(r))
				return nil
			})
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	})

	writer.Render()
	fmt.Println()
	return err
}

func convertToRow(r *spanner.Row) []interface{} {
	var row []interface{}

	for ix := range r.Size() {

		switch r.ColumnType(ix).Code {
		case spannerpb.TypeCode_BOOL:
			var v bool
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			row = append(row, v)
		case spannerpb.TypeCode_STRING:
			var v *string
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			if v == nil {
				row = append(row, nil)
			} else {
				row = append(row, *v)
			}

		case spannerpb.TypeCode_INT64:
			var v *int64
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			if v == nil {
				row = append(row, "nil")
			} else {
				row = append(row, *v)
			}

		case spannerpb.TypeCode_FLOAT64:
			var v *float64
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			if v == nil {
				row = append(row, "nil")
			} else {
				row = append(row, *v)
			}
		case spannerpb.TypeCode_FLOAT32:
			var v *float32
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			if v == nil {
				row = append(row, "nil")
			} else {
				row = append(row, *v)
			}
		case spannerpb.TypeCode_BYTES:
			var v []byte
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			row = append(row, hex.EncodeToString(v))
		case spannerpb.TypeCode_TIMESTAMP:
			var v *time.Time
			err := r.Column(ix, &v)
			if err != nil {
				row = append(row, err.Error())
			}
			if v == nil {
				row = append(row, "nil")
			} else {
				row = append(row, (*v).Format(time.RFC3339))
			}

		default:
			row = append(row, "Unknown type: "+r.ColumnType(ix).Code.String())
		}
	}
	return row
}
