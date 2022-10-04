package kitsync

import "strings"

func CleanString(s string) string {
	s = strings.ToValidUTF8(s, "?")
	s = strings.ReplaceAll(s, "\x00", "?")
	s = strings.TrimSpace(s)
	return s
}
