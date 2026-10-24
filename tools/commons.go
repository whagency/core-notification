package tools

import (
	"encoding/json"
	"strconv"
	"strings"
)

func TrimZeros(value any) string {
	maxPrecision := 8
	switch val := value.(type) {
	case float64:
		s := strconv.FormatFloat(val, 'f', maxPrecision, 64)
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
		return s
	case json.Number:
		return TrimZeros(val.String())
	case string:
		if strings.Contains(val, ".") {
			val = strings.TrimRight(val, "0")
			val = strings.TrimRight(val, ".")
		}
		return val
	default:
		return ""
	}
}
