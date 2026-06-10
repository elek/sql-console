package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"os"
)

// OutputFormat represents the format for query results
type OutputFormat string

const (
	// TableFormat represents the table output format
	TableFormat OutputFormat = "table"
	// CSVFormat represents the CSV output format
	CSVFormat OutputFormat = "csv"
	// JSONFormat represents the JSON output format
	JSONFormat OutputFormat = "json"
)

// ResultWriter interface for writing query results
type ResultWriter interface {
	SetHeader(columns []string)
	AppendRow(row []interface{})
	Render()
}

// TableWriter implements ResultWriter using table format.
// Header and rows are buffered so that, at render time, a single-row result
// can be transposed into a more readable key|value layout.
type TableWriter struct {
	columns []string
	rows    [][]interface{}
}

// NewTableWriter creates a new TableWriter
func NewTableWriter() ResultWriter {
	return &TableWriter{}
}

func (t *TableWriter) SetHeader(columns []string) {
	t.columns = columns
}

func (t *TableWriter) AppendRow(row []interface{}) {
	t.rows = append(t.rows, row)
}

func (t *TableWriter) Render() {
	tw := table.NewWriter()
	tw.SetOutputMirror(os.Stdout)

	if len(t.rows) == 1 {
		// Single result: render one column per row as a key|value table.
		tw.AppendHeader(table.Row{"Column", "Value"})
		row := t.rows[0]
		for i, col := range t.columns {
			var val interface{}
			if i < len(row) {
				val = row[i]
			}
			tw.AppendRow(table.Row{col, val})
		}
	} else {
		header := make(table.Row, len(t.columns))
		for i, col := range t.columns {
			header[i] = col
		}
		tw.AppendHeader(header)
		for _, row := range t.rows {
			tableRow := make(table.Row, len(row))
			for i, val := range row {
				tableRow[i] = val
			}
			tw.AppendRow(tableRow)
		}
	}

	tw.Render()
}

// CSVWriter implements ResultWriter using CSV format
type CSVWriter struct {
	writer *csv.Writer
}

// NewCSVWriter creates a new CSVWriter
func NewCSVWriter() ResultWriter {
	return &CSVWriter{
		writer: csv.NewWriter(os.Stdout),
	}
}

func (c *CSVWriter) SetHeader(columns []string) {
	c.writer.Write(columns)
}

func (c *CSVWriter) AppendRow(row []interface{}) {
	strRow := make([]string, len(row))
	for i, val := range row {
		if val == nil {
			strRow[i] = ""
		} else {
			strRow[i] = stringify(val)
		}
	}
	c.writer.Write(strRow)
}

func (c *CSVWriter) Render() {
	c.writer.Flush()
}

// JSONWriter implements ResultWriter using JSON format
type JSONWriter struct {
	columns []string
	rows    []map[string]interface{}
}

// NewJSONWriter creates a new JSONWriter
func NewJSONWriter() ResultWriter {
	return &JSONWriter{}
}

func (j *JSONWriter) SetHeader(columns []string) {
	j.columns = columns
}

func (j *JSONWriter) AppendRow(row []interface{}) {
	obj := make(map[string]interface{}, len(j.columns))
	for i, val := range row {
		if i < len(j.columns) {
			obj[j.columns[i]] = val
		}
	}
	j.rows = append(j.rows, obj)
}

func (j *JSONWriter) Render() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(j.rows)
}

// stringify converts any value to a string representation
func stringify(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

// GetResultWriter returns the appropriate ResultWriter based on format
func GetResultWriter(format string) ResultWriter {
	switch format {
	case string(CSVFormat):
		return NewCSVWriter()
	case string(JSONFormat):
		return NewJSONWriter()
	default:
		return NewTableWriter()
	}
}

// DatabaseClient defines the interface for database operations
type DatabaseClient interface {
	// Execute runs a query and returns the results
	Execute(ctx context.Context, query string) error

	ExecuteInTx(ctx context.Context, queries []string) error

	// Close releases any resources
	Close()

	// GetName returns a descriptive name for the connection
	GetName() string

	// ListTables lists all tables in the database
	ListTables(ctx context.Context) error
}
