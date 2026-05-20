package service

import (
	"path"
	"strings"
)

// MatchGlob checks if a file path matches a glob pattern.
// Supports:
//   - *     matches any sequence of non-/ characters
//   - **    matches zero or more path segments
//   - *.ext matches file extension
func MatchGlob(pattern, filePath string) bool {
	if pattern == "" {
		return false
	}
	if !strings.HasPrefix(filePath, "/") {
		return false
	}

	// Exact match (no wildcard)
	if !strings.Contains(pattern, "*") {
		return pattern == filePath
	}

	patternParts := splitPath(pattern)
	pathParts := splitPath(filePath)
	return matchParts(patternParts, pathParts)
}

func splitPath(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func matchParts(pattern, target []string) bool {
	for len(pattern) > 0 {
		seg := pattern[0]

		if seg == "**" {
			pattern = pattern[1:]
			// ** matches zero segments — try matching rest of pattern against current position
			if matchParts(pattern, target) {
				return true
			}
			// ** matches one or more segments — consume one target segment and retry
			for len(target) > 0 {
				target = target[1:]
				if matchParts(pattern, target) {
					return true
				}
			}
			return false
		}

		if len(target) == 0 {
			return false
		}

		matched, err := path.Match(seg, target[0])
		if err != nil || !matched {
			return false
		}
		pattern = pattern[1:]
		target = target[1:]
	}

	return len(target) == 0
}
