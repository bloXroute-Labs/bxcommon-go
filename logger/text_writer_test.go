package logger

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTextWriterMatchesConsoleWriter(t *testing.T) {
	caller := filepath.Join(t.TempDir(), "writer.go")
	tests := map[string]string{
		"fields":  `{"level":"info","time":1786377600123456789,"message":"submitted","signature":"abc","slot":123}`,
		"quoting": `{"level":"warn","time":"2026-08-10T12:00:00.000000","message":"bad \"request\"","error":"connection refused","empty":""}`,
		"nested":  `{"level":"debug","time":1786377600123456789,"message":"details","array":[1,"two"],"object":{"a":1,"b":true}}`,
		"caller":  `{"level":"error","time":1786377600123456789,"caller":"` + caller + `","message":"failed"}`,
	}

	for name, event := range tests {
		t.Run(name, func(t *testing.T) {
			var expected, actual bytes.Buffer
			legacy := newWriter(&expected, true)
			fast := newTextWriter(&actual)

			_, expectedErr := legacy.Write([]byte(event))
			_, actualErr := fast.Write([]byte(event))

			require.Equal(t, expectedErr, actualErr)
			require.Equal(t, expected.String(), actual.String())
		})
	}
}

func TestTextWriterAllocations(t *testing.T) {
	event := []byte(`{"level":"info","time":1786377600123456789,"message":"transaction submitted","accountID":"12345","blockHash":"abc","signature":"xyz","slot":123456,"validator":"validator-key"}`)
	writer := newTextWriter(io.Discard)

	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = writer.Write(event)
	})

	require.Zero(t, allocs)
}

func BenchmarkTextWriter(b *testing.B) {
	event := []byte(`{"level":"info","time":1786377600123456789,"message":"transaction submitted","accountID":"12345","blockHash":"abc","signature":"xyz","slot":123456,"validator":"validator-key"}`)

	b.Run("ConsoleWriter", func(b *testing.B) {
		writer := newWriter(io.Discard, true)
		b.ReportAllocs()
		for b.Loop() {
			_, _ = writer.Write(event)
		}
	})
	b.Run("TextWriter", func(b *testing.B) {
		writer := newTextWriter(io.Discard)
		_, _ = writer.Write(event) // Warm the pool.
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			_, _ = writer.Write(event)
		}
	})
}
