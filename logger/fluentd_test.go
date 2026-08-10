package logger

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bloXroute-Labs/fluent-logger-golang/fluent"
	"github.com/stretchr/testify/require"
)

type captureRawPoster struct {
	tm     time.Time
	record []byte
}

func (p *captureRawPoster) PostRawJSON(_ string, tm time.Time, record []byte) error {
	p.tm = tm
	p.record = append(p.record[:0], record...)
	return nil
}

func TestFluentJSONWriter(t *testing.T) {
	event := []byte(`{"level":"info","time":1786377600123456789,"message":"submitted \"fast\"","slot":123,"nested":{"ok":true},"timestamp":"old","instance":"old"}`)
	poster := &captureRawPoster{}
	writer := &fluentJSONWriter{poster: poster, instance: "node-1"}

	n, err := writer.Write(event)
	require.NoError(t, err)
	require.Equal(t, len(event), n)

	formatted := time.Unix(0, 1786377600123456789).UTC().Format(timestampFormat)
	var actual map[string]any
	require.NoError(t, json.Unmarshal(poster.record, &actual))
	require.Equal(t, map[string]any{
		"level": "INFO", "time": formatted, "timestamp": formatted,
		"message": `submitted "fast"`, "slot": float64(123),
		"nested": map[string]any{"ok": true}, "instance": "node-1",
	}, actual)
	require.Equal(t, time.Unix(0, 1786377600123456789).UTC(), poster.tm)
}

func TestFluentJSONWriterAllocations(t *testing.T) {
	event := []byte(`{"level":"info","time":1786377600123456789,"message":"transaction submitted","accountID":"12345","blockHash":"abc","signature":"xyz","slot":123456,"validator":"validator-key"}`)
	writer := &fluentJSONWriter{poster: noopRawPoster{}, instance: "node-1"}
	_, _ = writer.Write(event)

	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = writer.Write(event)
	})
	require.Zero(t, allocs)
}

type noopRawPoster struct{}

func (noopRawPoster) PostRawJSON(string, time.Time, []byte) error { return nil }

type encodingRawPoster struct{ data []byte }

func (p *encodingRawPoster) PostRawJSON(tag string, tm time.Time, record []byte) error {
	p.data = make([]byte, 0, len(tag)+len(record)+32)
	p.data = append(p.data, '[', '"')
	p.data = append(p.data, tag...)
	p.data = append(p.data, '"', ',')
	p.data = strconv.AppendInt(p.data, tm.Unix(), 10)
	p.data = append(p.data, ',')
	p.data = append(p.data, record...)
	p.data = append(p.data, `,{}`...)
	p.data = append(p.data, ']')
	return nil
}

func BenchmarkFluentDEncoding(b *testing.B) {
	event := []byte(`{"level":"info","time":1786377600123456789,"message":"transaction submitted","accountID":"12345","blockHash":"abc","signature":"xyz","slot":123456,"validator":"validator-key"}`)

	b.Run("ConsoleWriterAndEncodeData", func(b *testing.B) {
		fd := &fluent.Fluent{Config: fluent.Config{MarshalAsJSON: true}}
		writer := newWriter(io.Discard, true)
		writer.FormatPrepare = func(m map[string]interface{}) error {
			ns, _ := m["time"].(json.Number).Int64()
			tm := time.Unix(0, ns).UTC()
			formatted := tm.Format(timestampFormat)
			m["level"] = strings.ToUpper(m["level"].(string))
			m["time"], m["timestamp"], m["instance"] = formatted, formatted, "node-1"
			_, err := fd.EncodeData(fluentDTag, tm, m)
			return err
		}
		b.ReportAllocs()
		for b.Loop() {
			_, _ = writer.Write(event)
		}
	})

	b.Run("RawJSON", func(b *testing.B) {
		writer := &fluentJSONWriter{poster: &encodingRawPoster{}, instance: "node-1"}
		_, _ = writer.Write(event)
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			_, _ = writer.Write(event)
		}
	})
}
