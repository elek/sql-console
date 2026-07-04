package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseExactTimestamp(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	t.Run("recent timestamp is accepted", func(t *testing.T) {
		ts, err := parseExactTimestamp("2026-06-26T11:30:00Z", now)
		require.NoError(t, err)
		require.Equal(t, time.Date(2026, 6, 26, 11, 30, 0, 0, time.UTC), ts)
	})

	t.Run("too old timestamp is rejected", func(t *testing.T) {
		_, err := parseExactTimestamp("2026-06-01T13:20:05Z", now)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too old")
	})

	t.Run("future timestamp is rejected", func(t *testing.T) {
		_, err := parseExactTimestamp("2026-06-26T13:00:00Z", now)
		require.Error(t, err)
		require.Contains(t, err.Error(), "future")
	})

	t.Run("invalid format is rejected", func(t *testing.T) {
		_, err := parseExactTimestamp("not-a-timestamp", now)
		require.Error(t, err)
		require.Contains(t, err.Error(), "RFC3339")
	})
}
