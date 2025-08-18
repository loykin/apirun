package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func isJSON(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	if (strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}")) || (strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")) {
		var js json.RawMessage
		return json.Unmarshal([]byte(t), &js) == nil
	}
	return false
}

func anyToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	case float64:
		// Avoid scientific notation for integers
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		// Fallback to JSON
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		// Remove surrounding quotes if it's a basic JSON string
		b = bytes.TrimSpace(b)
		if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
			return string(b[1 : len(b)-1])
		}
		return string(b)
	}
}
