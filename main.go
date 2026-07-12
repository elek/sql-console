package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
)

func main() {
	ktx := kong.Parse(&Cli{})
	err := ktx.Run()
	if err != nil {
		log.Fatalf("Failed to run: %v", err)
	}
}

type Cli struct {
	Alias           string        `arg:"" optional:"" help:"Alias name from ~/.config/sql-console/alias"`
	SpannerInstance string        `name:"spanner" help:"Spanner instance, in the form of projects/{project}/instances/{instance}/databases/{database} or {project}/{instance}/{database}"`
	BigQueryProject string        `name:"bigquery" help:"BigQuery project ID"`
	Postgres        string        `name:"postgres" help:"PostgreSQL connection string (URL or DSN, e.g. postgres://user:pass@host:5432/db)"`
	Transaction     bool          `name:"transaction" short:"t" help:"Execute all queries in a single transaction"`
	OutputFormat    string        `name:"format" short:"f" help:"Output format (table|csv|json)" default:"table" enum:"table,csv,json"`
	Staleness       time.Duration `name:"staleness" help:"Staleness duration for Spanner stale reads (e.g. 10s, 1m)"`
	ExactTimestamp  string        `name:"exact-timestamp" help:"Exact timestamp for Spanner stale reads (RFC3339 format, e.g. 2006-01-02T15:04:05Z)"`
}

// Store outputFormat as a global variable for all DB clients to access
var outputFormat string

// resolveAlias looks up an alias in ~/.config/sql-console/alias
// Returns (type, connection-string, error)
func resolveAlias(alias string) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get home directory")
	}

	aliasPath := filepath.Join(home, ".config", "sql-console", "alias")
	file, err := os.Open(aliasPath)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to open alias file")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 && parts[0] == alias {
			return parts[1], parts[2], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", errors.Wrap(err, "failed to read alias file")
	}

	return "", "", errors.Errorf("alias %q not found", alias)
}

func (c *Cli) Run() error {
	// Set up appropriate client based on which flag was provided
	ctx := context.Background()
	
	// Set the global output format
	outputFormat = c.OutputFormat

	// Resolve alias if provided
	if c.Alias != "" {
		if c.SpannerInstance != "" || c.BigQueryProject != "" || c.Postgres != "" {
			return errors.New("Cannot specify both alias and --spanner/--bigquery/--postgres flags")
		}
		dbType, connStr, err := resolveAlias(c.Alias)
		if err != nil {
			return err
		}
		switch dbType {
		case "spanner":
			c.SpannerInstance = connStr
		case "bigquery":
			c.BigQueryProject = connStr
		case "postgres":
			c.Postgres = connStr
		default:
			return errors.Errorf("unknown database type %q for alias %q", dbType, c.Alias)
		}
	}

	var dbClient DatabaseClient
	var err error

	specified := 0
	if c.SpannerInstance != "" {
		specified++
	}
	if c.BigQueryProject != "" {
		specified++
	}
	if c.Postgres != "" {
		specified++
	}
	if specified > 1 {
		return errors.New("Cannot specify more than one of --spanner, --bigquery, --postgres")
	}

	if c.SpannerInstance != "" {
		// Handle Spanner connection string formatting
		parts := strings.Split(c.SpannerInstance, "/")
		if len(parts) != 6 && len(parts) != 3 {
			return errors.New(fmt.Sprintf("Invalid Spanner instance definition: %s", c.SpannerInstance))
		}
		prompt := c.SpannerInstance
		if len(parts) == 3 {
			c.SpannerInstance = fmt.Sprintf("projects/%s/instances/%s/databases/%s", parts[0], parts[1], parts[2])
		} else if len(parts) == 6 {
			prompt = fmt.Sprintf("%s/%s/%s", parts[1], parts[3], parts[5])
		} else {
			return errors.New(fmt.Sprintf("Invalid Spanner instance: %s", c.SpannerInstance))
		}

		// Check if both staleness and exact-timestamp are provided
		if c.Staleness > 0 && c.ExactTimestamp != "" {
			return errors.New("Cannot specify both --staleness and --exact-timestamp")
		}
		
		var exactTimestamp time.Time
		var useExactTimestamp bool

		if c.ExactTimestamp != "" {
			var err error
			exactTimestamp, err = parseExactTimestamp(c.ExactTimestamp, time.Now())
			if err != nil {
				return err
			}
			useExactTimestamp = true
		}
		
		dbClient, err = NewSpannerClient(ctx, c.SpannerInstance, prompt, c.Staleness, exactTimestamp, useExactTimestamp)
	} else if c.BigQueryProject != "" {
		dbClient, err = NewBigQueryClient(ctx, c.BigQueryProject)
	} else if c.Postgres != "" {
		dbClient, err = NewPostgresClient(ctx, c.Postgres)
	} else {
		return errors.New("One of --spanner, --bigquery or --postgres must be specified")
	}

	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}

	defer dbClient.Close()

	stat, _ := os.Stdin.Stat()

	var queries []string
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return errors.Wrap(err, "failed to read from stdin")
		}
		for _, line := range strings.Split(string(content), ";") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fmt.Println(line)
			if c.Transaction {
				queries = append(queries, line)
			} else {
				err := dbClient.Execute(ctx, string(line))
				if err != nil {
					return errors.WithStack(err)
				}
			}
		}
		if len(queries) > 0 {
			err := dbClient.ExecuteInTx(ctx, queries)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	}

	return Loop(dbClient.GetName(), func(query string) {
		err := dbClient.Execute(ctx, query)
		if err != nil {
			fmt.Printf("Failed to execute query: %v\n", err)
		}
	}, dbClient)
}
