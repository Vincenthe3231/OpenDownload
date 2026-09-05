package parser

import (
	"strconv"
	"strings"
)

func parseAttributes(value string) map[string]string {
	attrs := make(map[string]string)
	var key, parsedValue string
	inQuote := false
	state := 0

	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch == '=' && state == 0:
			state = 1
		case ch == '"':
			inQuote = !inQuote
		case ch == ',' && !inQuote:
			attrs[strings.TrimSpace(key)] = strings.TrimSpace(parsedValue)
			key = ""
			parsedValue = ""
			state = 0
		case state == 0:
			key += string(ch)
		case state == 1:
			parsedValue += string(ch)
		}
	}
	if key != "" {
		attrs[strings.TrimSpace(key)] = strings.TrimSpace(parsedValue)
	}
	return attrs
}

func parseResolution(value string) Resolution {
	parts := strings.SplitN(value, "x", 2)
	if len(parts) != 2 {
		return Resolution{}
	}
	width, _ := strconv.Atoi(parts[0])
	height, _ := strconv.Atoi(parts[1])
	return Resolution{Width: width, Height: height}
}
