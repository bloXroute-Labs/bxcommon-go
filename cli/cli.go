package cli

import (
	"strings"
)

func ExtractArgsToMap(argsString string) map[string]string {
	args := strings.Split(argsString, "--")
	argsMap := make(map[string]string)

	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		switch {
		case strings.Contains(arg, " "):
			// arg key value are seperated by space
			parts := strings.SplitN(arg, " ", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			argsMap[key] = value
		case strings.Contains(arg, "="):
			// arg key value are seperated by equals
			parts := strings.SplitN(arg, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			argsMap[key] = value
		default:
			// arg has only key
			key := strings.TrimSpace(arg)
			argsMap[key] = ""
		}
	}

	return argsMap
}
