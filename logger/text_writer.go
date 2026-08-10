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

	"github.com/rs/zerolog"
)

var textWriterPool = sync.Pool{New: func() any {
	return &textWriterState{
		buf:    make([]byte, 0, 1024),
		fields: make([]textField, 0, 16),
	}
}}

type textField struct {
	key, value []byte
}

type textWriterState struct {
	buf    []byte
	fields []textField
}

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
	state.buf, state.fields = state.buf[:0], state.fields[:0]
	defer func() {
		if cap(state.buf) <= 64*1024 && cap(state.fields) <= 256 {
			textWriterPool.Put(state)
		}
	}()

	fields, ok := parseTextFields(p, state.fields)
	if !ok {
		return w.fallback.Write(p)
	}

	var timestamp, level, caller, message []byte
	state.fields = state.fields[:0]
	for _, field := range fields {
		switch string(field.key) {
		case "time":
			timestamp = field.value
		case "level":
			level = field.value
		case "caller":
			caller = field.value
		case "message":
			message = field.value
		default:
			state.fields = append(state.fields, field)
		}
	}
	if len(timestamp) == 0 || !isJSONString(level) || !isJSONString(message) {
		return w.fallback.Write(p)
	}

	state.buf, ok = appendTextTimestamp(state.buf, timestamp)
	if !ok {
		return w.fallback.Write(p)
	}
	state.buf, ok = appendTextString(state.buf, level, " level=\"")
	if !ok {
		return w.fallback.Write(p)
	}
	if len(caller) > 0 {
		state.buf, ok = appendTextCaller(state.buf, caller)
		if !ok {
			return w.fallback.Write(p)
		}
	}
	state.buf, ok = appendTextString(state.buf, message, " msg=\"")
	if !ok {
		return w.fallback.Write(p)
	}

	slices.SortFunc(state.fields, func(left, right textField) int {
		if string(left.key) == zerolog.ErrorFieldName {
			return -1
		}
		if string(right.key) == zerolog.ErrorFieldName {
			return 1
		}
		return bytes.Compare(left.key, right.key)
	})
	for _, field := range state.fields {
		state.buf = append(state.buf, ' ')
		state.buf = append(state.buf, field.key...)
		state.buf = append(state.buf, '=')
		state.buf, ok = appendTextValue(state.buf, field.value)
		if !ok {
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

func appendTextTimestamp(dst, raw []byte) ([]byte, bool) {
	dst = append(dst, "time=\""...)
	if isJSONString(raw) {
		var ok bool
		dst, ok = appendTextUnquoted(dst, raw)
		if !ok {
			return dst, false
		}
	} else {
		ns, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			return dst, false
		}
		dst = time.Unix(0, ns).UTC().AppendFormat(dst, timestampFormat)
	}
	return append(dst, '"'), true
}

func appendTextString(dst, raw []byte, prefix string) ([]byte, bool) {
	if !isJSONString(raw) {
		return dst, false
	}
	dst = append(dst, prefix...)
	var ok bool
	dst, ok = appendTextUnquoted(dst, raw)
	return append(dst, '"'), ok
}

func appendTextCaller(dst, raw []byte) ([]byte, bool) {
	caller, ok := unquoteTextString(raw)
	if !ok || caller == "" {
		return dst, ok
	}
	if cwd, err := os.Getwd(); err == nil {
		if relative, err := filepath.Rel(cwd, caller); err == nil {
			caller = relative
		}
	}
	dst = append(dst, ' ')
	dst = append(dst, caller...)
	return append(dst, " >"...), true
}

func appendTextValue(dst, raw []byte) ([]byte, bool) {
	if !isJSONString(raw) {
		return append(dst, raw...), len(raw) > 0
	}
	if !bytes.ContainsRune(raw, '\\') {
		value := raw[1 : len(raw)-1]
		if needsTextQuote(value) {
			return append(dst, raw...), true
		}
		return append(dst, value...), true
	}
	value, ok := unquoteTextString(raw)
	if !ok {
		return dst, false
	}
	if needsTextQuote([]byte(value)) {
		return strconv.AppendQuote(dst, value), true
	}
	return append(dst, value...), true
}

func appendTextUnquoted(dst, raw []byte) ([]byte, bool) {
	if !bytes.ContainsRune(raw, '\\') {
		return append(dst, raw[1:len(raw)-1]...), true
	}
	value, ok := unquoteTextString(raw)
	return append(dst, value...), ok
}

func unquoteTextString(raw []byte) (string, bool) {
	if !isJSONString(raw) {
		return "", false
	}
	value, err := strconv.Unquote(string(raw))
	return value, err == nil
}

func isJSONString(raw []byte) bool {
	return len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"'
}

func needsTextQuote(value []byte) bool {
	for _, b := range value {
		if b < 0x20 || b > 0x7e || b == ' ' || b == '\\' || b == '"' {
			return true
		}
	}
	return false
}

func parseTextFields(data []byte, dst []textField) ([]textField, bool) {
	i := skipTextSpace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return dst, false
	}
	for i++; ; {
		i = skipTextSpace(data, i)
		if i >= len(data) {
			return dst, false
		}
		if data[i] == '}' {
			return dst, true
		}

		keyEnd, ok := scanTextString(data, i)
		if !ok || bytes.ContainsRune(data[i:keyEnd], '\\') {
			return dst, false
		}
		key := data[i+1 : keyEnd-1]
		i = skipTextSpace(data, keyEnd)
		if i >= len(data) || data[i] != ':' {
			return dst, false
		}
		i = skipTextSpace(data, i+1)
		valueStart := i
		i, ok = scanTextValue(data, i)
		if !ok {
			return dst, false
		}
		for _, field := range dst {
			if bytes.Equal(field.key, key) {
				return dst, false
			}
		}
		dst = append(dst, textField{key: key, value: data[valueStart:i]})

		i = skipTextSpace(data, i)
		if i >= len(data) {
			return dst, false
		}
		if data[i] == '}' {
			return dst, true
		}
		if data[i] != ',' {
			return dst, false
		}
		i++
	}
}

func scanTextValue(data []byte, start int) (int, bool) {
	if start >= len(data) {
		return start, false
	}
	if data[start] == '"' {
		return scanTextString(data, start)
	}
	if data[start] != '{' && data[start] != '[' {
		i := start
		for i < len(data) && data[i] != ',' && data[i] != '}' && !isTextSpace(data[i]) {
			i++
		}
		return i, i > start
	}

	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '"':
			end, ok := scanTextString(data, i)
			if !ok {
				return start, false
			}
			i = end - 1
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return start, false
}

func scanTextString(data []byte, start int) (int, bool) {
	if start >= len(data) || data[start] != '"' {
		return start, false
	}
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i + 1, true
		}
	}
	return start, false
}

func skipTextSpace(data []byte, i int) int {
	for i < len(data) && isTextSpace(data[i]) {
		i++
	}
	return i
}

func isTextSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
