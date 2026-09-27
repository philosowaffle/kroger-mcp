// Package logging configures the process-wide slog logger. The level comes
// from KROGER_MCP_LOG_LEVEL, and a TRACE level below DEBUG is available for
// per-request diagnostics that should stay off by default.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace sits below slog.LevelDebug for very verbose output.
const LevelTrace = slog.Level(-8)

// Setup installs a text slog handler on stderr as the default logger.
func Setup() {
	opts := &slog.HandlerOptions{
		Level: ParseLevel(os.Getenv("KROGER_MCP_LOG_LEVEL")),
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == LevelTrace {
					a.Value = slog.StringValue("TRACE")
				}
			}
			return a
		},
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
}

// Trace logs msg at LevelTrace on the default logger.
func Trace(msg string, args ...any) {
	slog.Log(context.Background(), LevelTrace, msg, args...)
}

// ParseLevel maps trace|debug|info|warn|error to a slog level. Anything else,
// including the empty string, is info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
