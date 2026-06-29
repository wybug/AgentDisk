package okf

import (
	"errors"
	"strings"
)

// reservedFileNames are the OKF file names that have special meaning within a
// bundle and may not be used for ordinary artifacts. The check is
// case-sensitive per the OKF spec.
var reservedFileNames = map[string]struct{}{
	"index.md": {},
	"log.md":   {},
}

// ErrMissingType is returned by Validate when a Frontmatter has no Type field.
// It is a sentinel so callers can branch on "missing type" versus other errors.
var ErrMissingType = errors.New("okf: frontmatter \"type\" is required")

// Validate checks the OKF invariants that this library enforces. Per the OKF
// specification Type is the only required field; everything else (including
// unknown keys) is tolerated. Type is considered present only if it contains
// non-whitespace characters after trimming.
func Validate(fm Frontmatter) error {
	if strings.TrimSpace(fm.Type) == "" {
		return ErrMissingType
	}
	return nil
}

// IsReservedName reports whether name is one of the OKF reserved file names
// ("index.md" or "log.md"). The comparison is case-sensitive: "INDEX.md" is
// NOT reserved and may be used as an ordinary artifact.
func IsReservedName(name string) bool {
	_, ok := reservedFileNames[name]
	return ok
}
