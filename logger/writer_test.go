package logger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func writeFileLines(t *testing.T, jsonLines bool) []string {
	t.Helper()

	fileName := filepath.Join(t.TempDir(), "test.log")
	fw := fileWriter(fileName, 1, 1, 1, zerolog.TraceLevel, jsonLines)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixNano
	l := zerolog.New(fw).With().Timestamp().Logger()
	l.Debug().Str("bundle_hash", "0xabc").Int("block_number", 7).Msg("first")
	l.Info().Msg("second")

	if err := fw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	f, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return lines
}

func TestFileWriterJSONLines(t *testing.T) {
	lines := writeFileLines(t, true)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), lines)
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line is not JSON: %v: %q", err, lines[0])
	}
	for key, want := range map[string]any{"level": "debug", "message": "first", "bundle_hash": "0xabc", "block_number": float64(7)} {
		if first[key] != want {
			t.Errorf("%s = %v, want %v", key, first[key], want)
		}
	}
	if _, ok := first["time"]; !ok {
		t.Errorf("time field missing: %q", lines[0])
	}
}

func TestFileWriterText(t *testing.T) {
	lines := writeFileLines(t, false)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], `msg="first"`) || !strings.Contains(lines[0], "bundle_hash=0xabc") {
		t.Errorf("unexpected text line: %q", lines[0])
	}
	if json.Valid([]byte(lines[0])) {
		t.Errorf("text mode wrote JSON: %q", lines[0])
	}
}
