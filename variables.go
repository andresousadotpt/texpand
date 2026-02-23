package main

import (
	"strings"
	"time"
)

// ResolveVars resolves global and match-level variables, processing them
// in order so later variables can reference earlier ones via {{name}}.
func ResolveVars(globalVars []VarDef, matchVars []VarDef, now time.Time) map[string]string {
	resolved := make(map[string]string)

	// Combine global + match vars; order matters (later refs earlier)
	allVars := make([]VarDef, 0, len(globalVars)+len(matchVars))
	allVars = append(allVars, globalVars...)
	allVars = append(allVars, matchVars...)

	for _, v := range allVars {
		if v.Type == "date" {
			t := now.Add(time.Duration(v.Params.Offset) * time.Second)
			// First expand {{refs}} to already-resolved values
			format := expandRefs(v.Params.Format, resolved)
			// Then replace strftime tokens with actual date values
			resolved[v.Name] = resolveDate(format, t)
		}
	}

	return resolved
}

// expandRefs replaces {{name}} placeholders with resolved values.
func expandRefs(s string, vars map[string]string) string {
	for name, val := range vars {
		s = strings.ReplaceAll(s, "{{"+name+"}}", val)
	}
	return s
}
