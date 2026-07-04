package main

import (
	"time"

	"github.com/pkg/errors"
)

// maxExactTimestampAge bounds how far in the past an --exact-timestamp may
// point. Spanner only keeps historical versions within the database's version
// retention period (1 hour by default), so exact-stale reads older than this
// are rejected by the server. We validate up front to fail fast with a clear
// message instead of entering the REPL and erroring on every query.
const maxExactTimestampAge = time.Hour

// parseExactTimestamp parses an RFC3339 timestamp for use as a Spanner exact
// stale read bound, validating that it is neither in the future nor older than
// the version retention period.
func parseExactTimestamp(s string, now time.Time) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "Invalid exact timestamp format. Please use RFC3339 format (e.g. 2006-01-02T15:04:05Z)")
	}
	if ts.After(now) {
		return time.Time{}, errors.Errorf("exact timestamp %s is in the future", s)
	}
	if now.Sub(ts) > maxExactTimestampAge {
		return time.Time{}, errors.Errorf("exact timestamp %s is too old: Spanner stale reads are limited to the version retention period (default %s)", s, maxExactTimestampAge)
	}
	return ts, nil
}
