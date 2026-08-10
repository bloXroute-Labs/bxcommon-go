package logger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/rs/zerolog"
)

var (
	timeKey    = []byte(zerolog.TimestampFieldName)
	levelKey   = []byte(zerolog.LevelFieldName)
	callerKey  = []byte(zerolog.CallerFieldName)
	messageKey = []byte(zerolog.MessageFieldName)
	errorKey   = []byte(zerolog.ErrorFieldName)
)

type textField struct {
	key, value []byte
	typ        jsonparser.ValueType
}

type textWriterState struct {
	buf, keys, scratch []byte
	fields             []textField
}

var textWriterPool = sync.Pool{New: func() any {
	return &textWriterState{buf: make([]byte, 0, 1024), fields: make([]textField, 0, 16)}
}}

// textWriter preserves the existing no-color console format without decoding
// each zerolog event into a map[string]interface{}.
type textWriter struct {
	out      io.Writer
	fallback zerolog.ConsoleWriter
}

func newTextWriter(out io.Writer) *textWriter {
	return &textWriter{out: out, fallback: newWriter(out, true)}
}

func (w *textWriter) Write(p []byte) (int, error) {
	state := textWriterPool.Get().(*textWriterState)
	state.buf, state.keys, state.fields = state.buf[:0], state.keys[:0], state.fields[:0]
	if cap(state.keys) < len(p) {
		state.keys = make([]byte, 0, len(p))
	}
	if cap(state.scratch) < len(p) {
		state.scratch = make([]byte, 0, len(p))
	}
	defer func() {
		if cap(state.buf) <= 64*1024 {
			textWriterPool.Put(state)
		}
	}()

	err := jsonparser.ObjectEach(p, func(key, value []byte, typ jsonparser.ValueType, _ int) error {
		start := len(state.keys)
		state.keys = append(state.keys, key...)
		state.fields = append(state.fields, textField{state.keys[start:], value, typ})
		return nil
	})
	if err != nil {
		return w.fallback.Write(p)
	}

	var timestamp, level, caller, message textField
	fields := state.fields[:0]
	for _, field := range state.fields {
		switch {
		case bytes.Equal(field.key, timeKey):
			timestamp = field
		case bytes.Equal(field.key, levelKey):
			level = field
		case bytes.Equal(field.key, callerKey):
			caller = field
		case bytes.Equal(field.key, messageKey):
			message = field
		default:
			fields = append(fields, field)
		}
	}
	if len(timestamp.value) == 0 || level.typ != jsonparser.String || message.typ != jsonparser.String {
		return w.fallback.Write(p)
	}

	if state.buf, err = appendTextTimestamp(state.buf, timestamp, state.scratch); err != nil {
		return w.fallback.Write(p)
	}
	if state.buf, err = appendTextString(state.buf, ` level="`, level.value, state.scratch); err != nil {
		return w.fallback.Write(p)
	}
	if len(caller.value) > 0 {
		if state.buf, err = appendTextCaller(state.buf, caller, state.scratch); err != nil {
			return w.fallback.Write(p)
		}
	}
	if state.buf, err = appendTextString(state.buf, ` msg="`, message.value, state.scratch); err != nil {
		return w.fallback.Write(p)
	}

	slices.SortFunc(fields, func(left, right textField) int {
		if bytes.Equal(left.key, errorKey) {
			return -1
		}
		if bytes.Equal(right.key, errorKey) {
			return 1
		}
		return bytes.Compare(left.key, right.key)
	})
	for _, field := range fields {
		state.buf = append(state.buf, ' ')
		state.buf = append(state.buf, field.key...)
		state.buf = append(state.buf, '=')
		if state.buf, err = appendTextValue(state.buf, field, state.scratch); err != nil {
			return w.fallback.Write(p)
		}
	}
	state.buf = append(state.buf, '\n')

	n, err := w.out.Write(state.buf)
	if err == nil && n != len(state.buf) {
		err = io.ErrShortWrite
	}
	return len(p), err
}

func (w *textWriter) Close() error {
	if closer, ok := w.out.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func appendTextTimestamp(dst []byte, field textField, scratch []byte) ([]byte, error) {
	dst = append(dst, `time="`...)
	if field.typ == jsonparser.String {
		value, err := jsonparser.Unescape(field.value, scratch)
		return append(append(dst, value...), '"'), err
	}
	ns, err := jsonparser.ParseInt(field.value)
	if err != nil {
		return dst, err
	}
	dst = time.Unix(0, ns).UTC().AppendFormat(dst, timestampFormat)
	return append(dst, '"'), nil
}

func appendTextString(dst []byte, prefix string, raw, scratch []byte) ([]byte, error) {
	value, err := jsonparser.Unescape(raw, scratch)
	dst = append(dst, prefix...)
	return append(append(dst, value...), '"'), err
}

func appendTextCaller(dst []byte, field textField, scratch []byte) ([]byte, error) {
	value, err := jsonparser.Unescape(field.value, scratch)
	if err != nil || len(value) == 0 {
		return dst, err
	}
	caller := string(value)
	if cwd, err := os.Getwd(); err == nil {
		if relative, err := filepath.Rel(cwd, caller); err == nil {
			caller = relative
		}
	}
	dst = append(dst, ' ')
	dst = append(dst, caller...)
	return append(dst, " >"...), nil
}

func appendTextValue(dst []byte, field textField, scratch []byte) ([]byte, error) {
	if field.typ != jsonparser.String {
		return append(dst, field.value...), nil
	}
	value, err := jsonparser.Unescape(field.value, scratch)
	if err != nil {
		return dst, err
	}
	if needsTextQuote(value) {
		return strconv.AppendQuote(dst, string(value)), nil
	}
	return append(dst, value...), nil
}

func needsTextQuote(value []byte) bool {
	for _, b := range value {
		if b < 0x20 || b > 0x7e || b == ' ' || b == '\\' || b == '"' {
			return true
		}
	}
	return false
}
