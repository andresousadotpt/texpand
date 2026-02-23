package main

import (
	"strings"
	"time"
)

// resolveDate replaces strftime format tokens in the string with their
// corresponding time values. This avoids using Go's time.Format on
// arbitrary strings, which could misinterpret literal characters as
// layout tokens.
func resolveDate(format string, t time.Time) string {
	replacements := [][2]string{
		{"%Y", t.Format("2006")},
		{"%m", t.Format("01")},
		{"%d", t.Format("02")},
		{"%H", t.Format("15")},
		{"%I", t.Format("03")},
		{"%M", t.Format("04")},
		{"%S", t.Format("05")},
		{"%p", t.Format("PM")},
		{"%a", t.Format("Mon")},
		{"%A", t.Format("Monday")},
		{"%b", t.Format("Jan")},
		{"%B", t.Format("January")},
	}

	result := format
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r[0], r[1])
	}
	return result
}
