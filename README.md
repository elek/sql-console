Simple (and limited) SQL console for Google Spanner and BigQuery.

Usage:

```
sql-console --spanner=my_proect/my_instance/my_db
```

or:

```
sql-console --spanner=projects/my_proect/instances/my_instance/databases/my_db
```

or:

```
sql-console --bigquery=my_project
```

You can also pipe SQL commands:

```
cat /tmp/foo.sql | sql-console --spanner=...
```

Options:

- `--format` or `-f`: Output format (table|csv), default is table
- `--transaction` or `-t`: Execute all queries in a single transaction
- `--staleness`: Staleness duration for Spanner stale reads (e.g. 10s, 1m)

Example with CSV output:

```
sql-console --spanner=my_project/my_instance/my_db --format=csv
```
