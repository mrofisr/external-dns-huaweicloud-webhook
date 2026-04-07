/*
Copyright 2023 GleSYS AB

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"log/slog"
	"os"
	"strconv"
)

// Init configures the global slog logger from LOG_LEVEL and LOG_FORMAT environment variables.
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	if s == "" {
		return slog.LevelInfo
	}
	// Support numeric levels for backward compat.
	if n, err := strconv.Atoi(s); err == nil {
		switch {
		case n >= 6:
			return slog.LevelDebug
		case n >= 4:
			return slog.LevelInfo
		case n >= 2:
			return slog.LevelWarn
		default:
			return slog.LevelError
		}
	}
	switch s {
	case "debug", "DEBUG", "trace", "TRACE":
		return slog.LevelDebug
	case "info", "INFO":
		return slog.LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return slog.LevelWarn
	case "error", "ERROR", "fatal", "FATAL", "panic", "PANIC":
		return slog.LevelError
	default:
		slog.Warn("unknown log level, defaulting to info", "level", s)
		return slog.LevelInfo
	}
}
