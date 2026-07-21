package logger

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const timestampFormat = "2006-01-02T15:04:05.000000"

func formatTimestamp() zerolog.Formatter {
	return func(i interface{}) string {
		if num, ok := i.(json.Number); ok {
			ns, err := num.Int64()
			if err == nil {
				return fmt.Sprintf("time=\"%s\"", time.Unix(0, ns).UTC().Format(timestampFormat))
			}
		}
		return fmt.Sprintf("time=\"%s\"", i)
	}
}

func formatLevel(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		if ll, ok := i.(string); ok {
			if noColor {
				return fmt.Sprintf("level=\"%s\"", ll)
			}
			level, err := zerolog.ParseLevel(ll)
			if err != nil {
				level = zerolog.NoLevel
			}
			return fmt.Sprintf("level=\"\x1b[%dm%v\x1b[0m\"", zerolog.LevelColors[level], ll)
		}
		if i == nil {
			return "???"
		}
		return strings.ToUpper(fmt.Sprintf("%s", i))
	}
}
