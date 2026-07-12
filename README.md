Simple (and limited) SQL console for Google Spanner, Google BigQuery, PostgreSQL, TiDB, and ClickHouse.

Usage:

```
sql-console --spanner=my_project/my_instance/my_db
```

or:

```
sql-console --spanner=projects/my_project/instances/my_instance/databases/my_db
```

or:

```
sql-console --bigquery=my_project
```

```
sql-console --postgres="postgres://user:pass@host:5432/db"
```

```
sql-console --tidb="user:pass@tcp(host:4000)/dbname"
```

```
sql-console --clickhouse="clickhouse://user:pass@host:9000/dbname"
```

You can also pipe SQL commands:

```
cat /tmp/foo.sql | sql-console --spanner=...
```

Connection aliases can be stored in `~/.config/sql-console/alias`, one per line
as `NAME TYPE CONNECTION-STRING` (type is one of `spanner`, `bigquery`,
`postgres`, `tidb`, `clickhouse`), then used as `sql-console NAME`.

Options:

- `--format` or `-f`: Output format (table|csv|json), default is table
- `--transaction` or `-t`: Execute all queries in a single transaction
- `--staleness`: Staleness duration for Spanner stale reads (e.g. 10s, 1m)
- `--exact-timestamp`: Exact timestamp for Spanner stale reads (RFC3339)

Example with CSV output:

```
sql-console --spanner=my_project/my_instance/my_db --format=csv
```
