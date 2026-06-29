package okf

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// frontmatterDelimiter is the fence that opens and closes a YAML frontmatter
// block at the very top of an OKF markdown file.
const frontmatterDelimiter = "---"

// Parse splits an OKF markdown document into its YAML frontmatter and body.
//
// A document with no frontmatter (no leading "---\n") yields a zero Frontmatter,
// the entire input as body, and a nil error. A document whose first line is the
// fence but with no closing fence returns an error so callers can distinguish a
// malformed file from a plain markdown file.
func Parse(content []byte) (Frontmatter, []byte, error) {
	var fm Frontmatter
	// Fast path: documents that do not start with the fence are plain bodies.
	// This is the common case for body-only fragments and keeps Parse cheap.
	if !startsWithFence(content) {
		return fm, content, nil
	}

	// Drop the opening fence line, then look for the closing fence.
	afterOpen := dropFirstLine(content)
	closeIdx := findFenceLine(afterOpen)
	if closeIdx < 0 {
		return fm, nil, errors.New("okf: frontmatter not closed: missing closing \"---\" delimiter")
	}
	fmBytes := afterOpen[:closeIdx]
	// The body starts after the closing fence line. dropFirstLine consumes the
	// newline that terminated the closing fence. By OKF convention the
	// frontmatter and body are separated by one blank line; we drop exactly
	// that single leading newline from body so callers receive the body as
	// written, and Serialize can re-insert the blank line deterministically.
	body := dropFirstLine(afterOpen[closeIdx:])
	body = trimOneLeadingNewline(body)

	// An empty frontmatter block (just two fences) is legal but unusual; treat
	// it as a zero Frontmatter so callers see Type == "" and can decide.
	if len(bytes.TrimSpace(fmBytes)) == 0 {
		return fm, body, nil
	}

	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return Frontmatter{}, nil, fmt.Errorf("okf: invalid frontmatter YAML: %w", err)
	}
	return fm, body, nil
}

// Serialize renders a Frontmatter and body as an OKF markdown document with a
// leading YAML frontmatter block delimited by "---" fences. Unknown keys in
// fm.Extra are emitted alongside the known fields so round-trips are lossless.
//
// Serialize always emits a frontmatter block (even for an empty Frontmatter)
// because the body alone carries no type information and OKF requires Type.
func Serialize(fm Frontmatter, body []byte) ([]byte, error) {
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("okf: marshal frontmatter: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(frontmatterDelimiter)
	buf.WriteByte('\n')
	// yaml.Marshal already terminates with a trailing newline; write as-is so
	// nested/complex extra values render correctly.
	buf.Write(fmBytes)
	buf.WriteString(frontmatterDelimiter)
	buf.WriteByte('\n')
	// Guarantee exactly one blank line of separation between the closing fence
	// and the body when the body is non-empty, matching common OKF files and
	// making Serialize/Parse round-trips byte-stable.
	body = trimOneLeadingNewline(body)
	if len(body) > 0 {
		buf.WriteByte('\n')
		buf.Write(body)
		// Ensure the document ends with a newline for POSIX-friendly tooling.
		if !bytes.HasSuffix(body, []byte("\n")) {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

// trimOneLeadingNewline removes at most one leading "\n" (or "\r\n") from b.
// It is used to absorb the conventional blank line that separates the
// frontmatter fence from the body, so Serialize can re-emit it deterministically.
func trimOneLeadingNewline(b []byte) []byte {
	if len(b) > 0 && b[0] == '\n' {
		return b[1:]
	}
	if len(b) >= 2 && b[0] == '\r' && b[1] == '\n' {
		return b[2:]
	}
	return b
}

// startsWithFence reports whether content begins with the frontmatter fence on
// its own first line. We require the fence to be followed by a newline (or be
// the entire content) so a file starting with "---foo" is not misread.
func startsWithFence(content []byte) bool {
	const prefix = frontmatterDelimiter
	if len(content) < len(prefix) {
		return false
	}
	if !bytes.Equal(content[:len(prefix)], []byte(prefix)) {
		return false
	}
	rest := content[len(prefix):]
	if len(rest) == 0 {
		return true
	}
	// Allow optional "\r\n" line endings after the fence for Windows-authored
	// files; the body itself is preserved verbatim.
	if rest[0] == '\n' {
		return true
	}
	if len(rest) >= 2 && rest[0] == '\r' && rest[1] == '\n' {
		return true
	}
	return false
}

// findFenceLine returns the byte offset of the first line in content that is
// exactly the frontmatter delimiter (optionally followed by trailing spaces or
// a CR), or -1 if none exists. The offset points at the start of the fence so
// callers can slice [0:offset] to get the frontmatter payload.
func findFenceLine(content []byte) int {
	pos := 0
	for pos < len(content) {
		nl := bytes.IndexByte(content[pos:], '\n')
		var line []byte
		var lineEnd int
		if nl < 0 {
			line = content[pos:]
			lineEnd = len(content)
		} else {
			line = content[pos : pos+nl]
			lineEnd = pos + nl
		}
		trimmed := bytes.TrimRight(line, " \t\r")
		if len(trimmed) == len(frontmatterDelimiter) && string(trimmed) == frontmatterDelimiter {
			return pos
		}
		if nl < 0 {
			break
		}
		pos = lineEnd + 1
	}
	return -1
}

// dropFirstLine returns content with its first line removed. If content has no
// newline, the result is empty. The leading newline that terminated the first
// line is consumed, so the result begins at the second line's first byte.
func dropFirstLine(content []byte) []byte {
	nl := bytes.IndexByte(content, '\n')
	if nl < 0 {
		return nil
	}
	return content[nl+1:]
}
