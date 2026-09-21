//go:build integration

package repository

import (
	"io"
	"log/slog"
	"testing"
)

func testSlogLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
