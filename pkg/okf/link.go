package okf

import (
	"path"
	"path/filepath"
	"strings"
)

const bundlePrefix = "/bundle/"

// ParseBundleRelative inspects a link and, if it is a bundle-relative link
// (i.e. of the form "/bundle/<rel>"), returns the relative path with the
// prefix stripped and ok=true.
//
// Non bundle-relative links return ("", false). This includes external URLs
// (https://, http://, mailto:), pure anchor fragments (#foo), and ordinary
// relative links (./foo.md) — callers handle those via Resolve. The empty
// string is also non-bundle-relative.
//
// Trailing slashes are preserved; a bare "/bundle/" yields the empty relative
// path with ok=true, which callers may reject if a non-empty path is required.
func ParseBundleRelative(link string) (string, bool) {
	if link == bundlePrefix {
		return "", true
	}
	if !strings.HasPrefix(link, bundlePrefix) {
		return "", false
	}
	rel := strings.TrimPrefix(link, bundlePrefix)
	return rel, true
}

// isExternalOrFragment reports whether link is one of the link forms Resolve
// must not touch: absolute external URLs, scheme URLs (mailto:, tel:, ftp:),
// or pure anchor fragments. These are returned verbatim by Resolve.
func isExternalOrFragment(link string) bool {
	if link == "" {
		return true
	}
	if strings.HasPrefix(link, "#") {
		return true
	}
	// Scheme-prefixed URLs such as "https://", "mailto:foo@bar", "tel:+1...".
	// Only treat as external when there is a scheme; relative Windows-style
	// "C:\foo" does not match here.
	if i := strings.Index(link, "://"); i > 0 {
		return true
	}
	if colon := strings.Index(link, ":"); colon > 0 {
		// Allow a leading ":" only as part of a scheme. A bare "a:b" without a
		// scheme is treated as relative per OKF. Detect schemes by looking for
		// an alpha-first segment terminated by ":" with no slash before it.
		scheme := link[:colon]
		if isSchemeLike(scheme) {
			return true
		}
	}
	return false
}

// isSchemeLike reports whether s looks like a URL scheme: non-empty, ASCII
// letter first, followed by letters/digits/+/-/. per RFC 3986.
func isSchemeLike(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isASCIILetter(r) {
				return false
			}
			continue
		}
		if !isSchemeChar(r) {
			return false
		}
	}
	return true
}

// Resolve resolves an OKF relative link against the path of the file that
// contains it.
//
// currentPath is the bundle-relative path of the referencing document (e.g.
// "concepts/gemma.md"). relPath is the link as written, which may start with
// "./", "../", or be bare. The result is normalized with path.Clean and always
// uses forward slashes, matching the OKF requirement that bundle-relative
// paths use "/".
//
// External URLs (https://, http://, mailto:, tel:, ftp://, ...), pure anchor
// fragments (#section), and bundle-relative links (/bundle/...) are returned
// unchanged. Empty input yields ".".
//
// Windows-style backslashes in relPath are normalized to forward slashes before
// resolution so documents authored on Windows still resolve cleanly.
func Resolve(relPath, currentPath string) string {
	if relPath == "" {
		return "."
	}
	// Bundle-relative links are absolute within the bundle; hand them back
	// unchanged so callers can route them through ParseBundleRelative.
	if ok := strings.HasPrefix(relPath, bundlePrefix); ok {
		return relPath
	}
	if isExternalOrFragment(relPath) {
		return relPath
	}

	// Normalize Windows backslashes to forward slashes up front; OKF mandates
	// "/" and we want to be forgiving on input.
	rel := normalizeSlashes(relPath)
	cur := normalizeSlashes(currentPath)

	// Treat the current path as a directory: drop the file name so "./x" or
	// "../x" resolve relative to the document's folder. path.Dir never returns
	// "" for any input (it returns "." for the empty string and for bare file
	// names), so no further base-dir fallback is required.
	base := path.Dir(cur)

	joined := path.Join(base, rel)
	// path.Join collapses empty input to ".", and our empty-rel short-circuit
	// above guarantees rel is non-empty here, so joined is always non-empty.
	return joined
}

// normalizeSlashes converts any backslashes in s to forward slashes. This makes
// Windows-authored links like "..\summary.md" behave like "../summary.md".
func normalizeSlashes(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	// filepath.ToSlash only touches the OS separator, which on a non-Windows
	// build host is "/", so do the replacement explicitly to be portable.
	return filepath.ToSlash(strings.ReplaceAll(s, "\\", "/"))
}

// isASCIILetter reports whether r is an ASCII letter a-z or A-Z, the allowed
// first character of a URL scheme per RFC 3986. Kept as a helper so the scheme
// check reads without De Morgan negations that confuse static analysis.
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isASCIIDigit reports whether r is an ASCII digit 0-9.
func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isSchemeChar reports whether r may appear after the first character of a URL
// scheme (letters, digits, '+', '-', '.') per RFC 3986.
func isSchemeChar(r rune) bool {
	return isASCIILetter(r) || isASCIIDigit(r) || r == '+' || r == '-' || r == '.'
}
