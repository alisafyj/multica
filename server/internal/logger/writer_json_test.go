package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewWriterLoggerDefaultJSON(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	t.Setenv("MULTICA_DAEMON_LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "debug")

	var buf bytes.Buffer
	log := NewWriterLoggerDefault("daemon", &buf)
	log.Debug("debug-line",
		"string", "value",
		"number", 42,
		"flag", true,
		"group", slog.GroupValue(slog.String("nested", "present")),
	)
	slog.Info("global-line", "origin", "global")
	log.Warn("warn-line")
	log.Error("error-line")

	records := decodeJSONLogRecords(t, buf.String())
	if len(records) != 4 {
		t.Fatalf("decoded %d JSON records, want 4: %q", len(records), buf.String())
	}

	byMessage := make(map[string]map[string]any, len(records))
	for _, record := range records {
		message, ok := record["msg"].(string)
		if !ok {
			t.Fatalf("record missing string msg: %#v", record)
		}
		byMessage[message] = record

		timestamp, ok := record["time"].(string)
		if !ok {
			t.Fatalf("record %q missing string time: %#v", message, record)
		}
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			t.Errorf("record %q time %q is not full RFC3339 time: %v", message, timestamp, err)
		}
	}

	wantLevels := map[string]string{
		"debug-line":  "DEBUG",
		"global-line": "INFO",
		"warn-line":   "WARN",
		"error-line":  "ERROR",
	}
	for message, wantLevel := range wantLevels {
		record, ok := byMessage[message]
		if !ok {
			t.Errorf("missing %q record", message)
			continue
		}
		if got := record["level"]; got != wantLevel {
			t.Errorf("record %q level = %v, want %q", message, got, wantLevel)
		}
	}

	debug := byMessage["debug-line"]
	if got := debug["component"]; got != "daemon" {
		t.Errorf("component = %v, want daemon", got)
	}
	if got := debug["string"]; got != "value" {
		t.Errorf("string attr = %v, want value", got)
	}
	if got := debug["number"]; got != float64(42) {
		t.Errorf("number attr = %v, want 42", got)
	}
	if got := debug["flag"]; got != true {
		t.Errorf("flag attr = %v, want true", got)
	}
	group, ok := debug["group"].(map[string]any)
	if !ok || group["nested"] != "present" {
		t.Errorf("group attr = %#v, want nested=present", debug["group"])
	}
	if got := byMessage["global-line"]["origin"]; got != "global" {
		t.Errorf("global logger attr = %v, want global", got)
	}
}

func TestNewWriterLoggerDefaultJSONRespectsLogLevel(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	t.Setenv("MULTICA_DAEMON_LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "warn")

	var buf bytes.Buffer
	log := NewWriterLoggerDefault("daemon", &buf)
	log.Info("filtered-info")
	log.Warn("kept-warn")
	slog.Error("kept-global-error")

	records := decodeJSONLogRecords(t, buf.String())
	if len(records) != 2 {
		t.Fatalf("decoded %d JSON records, want 2: %q", len(records), buf.String())
	}
	if records[0]["msg"] != "kept-warn" || records[0]["level"] != "WARN" {
		t.Errorf("first record = %#v, want kept WARN record", records[0])
	}
	if records[1]["msg"] != "kept-global-error" || records[1]["level"] != "ERROR" {
		t.Errorf("second record = %#v, want kept global ERROR record", records[1])
	}
}

func TestNewWriterLoggerDefaultTintFallback(t *testing.T) {
	for _, test := range []struct {
		name   string
		format *string
	}{
		{name: "environment absent"},
		{name: "unsupported value", format: stringPointer("xml")},
	} {
		t.Run(test.name, func(t *testing.T) {
			previous := slog.Default()
			t.Cleanup(func() { slog.SetDefault(previous) })
			if test.format == nil {
				unsetenvForTest(t, "MULTICA_DAEMON_LOG_FORMAT")
			} else {
				t.Setenv("MULTICA_DAEMON_LOG_FORMAT", *test.format)
			}
			t.Setenv("LOG_LEVEL", "info")

			var buf bytes.Buffer
			log := NewWriterLoggerDefault("daemon", &buf)
			log.Info("tint-line", "code", 42)

			output := buf.String()
			if strings.HasPrefix(output, "{") {
				t.Fatalf("output unexpectedly uses JSON for %s: %q", test.name, output)
			}
			if len(output) < len("15:04:05.000") {
				t.Fatalf("output too short for tint timestamp: %q", output)
			}
			if _, err := time.Parse("15:04:05.000", output[:len("15:04:05.000")]); err != nil {
				t.Errorf("output does not retain tint short time: %q: %v", output, err)
			}
			for _, want := range []string{"tint-line", "component=daemon", "code=42"} {
				if !strings.Contains(output, want) {
					t.Errorf("tint output missing %q: %q", want, output)
				}
			}
			if strings.Contains(output, "\x1b[") {
				t.Errorf("tint output contains ANSI color escapes: %q", output)
			}
		})
	}
}

func decodeJSONLogRecords(t *testing.T, output string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not valid JSON: %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func unsetenvForTest(t *testing.T, key string) {
	t.Helper()

	value, present := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if present {
			if err := os.Setenv(key, value); err != nil {
				t.Errorf("restore %s: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Errorf("restore absent %s: %v", key, err)
		}
	})
}

func stringPointer(value string) *string {
	return &value
}
