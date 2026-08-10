package logger

import (
	"bytes"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/bloXroute-Labs/fluent-logger-golang/fluent"
	"github.com/buger/jsonparser"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
)

const (
	fluentDTag         = "bx.go.log"
	defaultBufferLimit = 32 * 1024
)

var (
	once   sync.Once
	nodeID string

	fluentTimeKey      = []byte(zerolog.TimestampFieldName)
	fluentLevelKey     = []byte(zerolog.LevelFieldName)
	fluentTimestampKey = []byte("timestamp")
	fluentInstanceKey  = []byte("instance")
	fluentBufferPool   = sync.Pool{New: func() any { return &fluentBuffer{buf: make([]byte, 0, 1024)} }}
	errFluentEvent     = errors.New("invalid zerolog event for fluentd")
)

// SetNodeID sets the node ID for the fluentd writer
func SetNodeID(id string) {
	once.Do(func() {
		nodeID = id
	})
}

type rawJSONPoster interface {
	PostRawJSON(string, time.Time, []byte) error
}

type fluentJSONWriter struct {
	poster   rawJSONPoster
	instance string
}

type fluentBuffer struct{ buf []byte }

func (w *fluentJSONWriter) Write(p []byte) (int, error) {
	state := fluentBufferPool.Get().(*fluentBuffer)
	buf := state.buf[:0]
	defer func() {
		if cap(buf) <= 64*1024 {
			state.buf = buf
			fluentBufferPool.Put(state)
		}
	}()

	buf = append(buf, '{')
	first := true
	var tm time.Time
	err := jsonparser.ObjectEach(p, func(key, value []byte, typ jsonparser.ValueType, _ int) error {
		if bytes.Equal(key, fluentTimestampKey) || (w.instance != "" && bytes.Equal(key, fluentInstanceKey)) {
			return nil
		}
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = appendFluentKey(buf, key)

		switch {
		case bytes.Equal(key, fluentTimeKey):
			if typ != jsonparser.Number {
				return errFluentEvent
			}
			ns, err := jsonparser.ParseInt(value)
			if err != nil {
				return err
			}
			tm = time.Unix(0, ns).UTC()
			buf = append(buf, '"')
			buf = tm.AppendFormat(buf, timestampFormat)
			buf = append(buf, '"')
		case bytes.Equal(key, fluentLevelKey):
			if typ != jsonparser.String {
				return errFluentEvent
			}
			buf = append(buf, '"')
			for _, b := range value {
				if b >= 'a' && b <= 'z' {
					b -= 'a' - 'A'
				}
				buf = append(buf, b)
			}
			buf = append(buf, '"')
		default:
			if typ == jsonparser.String {
				buf = append(buf, '"')
			}
			buf = append(buf, value...)
			if typ == jsonparser.String {
				buf = append(buf, '"')
			}
		}
		return nil
	})
	if err != nil || tm.IsZero() {
		return 0, errFluentEvent
	}

	buf = append(buf, `,"timestamp":"`...)
	buf = tm.AppendFormat(buf, timestampFormat)
	buf = append(buf, '"')
	if w.instance != "" {
		buf = append(buf, `,"instance":`...)
		buf = strconv.AppendQuote(buf, w.instance)
	}
	buf = append(buf, '}')

	if err := w.poster.PostRawJSON(fluentDTag, tm, buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

func appendFluentKey(dst, key []byte) []byte {
	for _, b := range key {
		if b < 0x20 || b == '\\' || b == '"' {
			return append(strconv.AppendQuote(dst, string(key)), ':')
		}
	}
	dst = append(dst, '"')
	dst = append(dst, key...)
	return append(dst, '"', ':')
}

// fluentDWriter returns a writer that writes to fluentd
func fluentDWriter(fluentDHost string, level zerolog.Level) (*levelWriter, error) {
	fd, err := fluent.New(fluent.Config{
		FluentPort:    24224,
		FluentHost:    fluentDHost,
		BufferLimit:   defaultBufferLimit,
		Async:         true,
		MarshalAsJSON: true,
	})
	if err != nil {
		return nil, err
	}

	return &levelWriter{
		WriteCloser: diode.NewWriter(&fluentJSONWriter{poster: fd, instance: nodeID}, backLog, 0, func(int) {}),
		minLevel:    zerolog.TraceLevel,
		maxLevel:    zerolog.PanicLevel,
		systemLevel: level,
	}, nil
}
