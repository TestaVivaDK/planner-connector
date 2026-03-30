package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

var Log *slog.Logger

func Init(verbose bool) {
	logDir := "logs"
	_ = os.MkdirAll(logDir, 0o700)
	logPath := filepath.Join(logDir, "planner-mcp.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		Log = slog.New(slog.NewTextHandler(io.Discard, nil))
		return
	}

	var w io.Writer = f
	if verbose {
		w = io.MultiWriter(f, os.Stderr)
	}
	Log = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
