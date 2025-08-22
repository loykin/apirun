package common

import "strings"

func HeaderOrDefault(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "Authorization"
	}
	return h
}
