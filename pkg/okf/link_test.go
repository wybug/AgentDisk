package okf

import (
	"testing"
)

func TestParseBundleRelative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		link    string
		wantRel string
		wantOK  bool
	}{
		{"/bundle/foo/bar.md", "foo/bar.md", true},
		{"/bundle/gemma.md", "gemma.md", true},
		{"/bundle/", "", true},
		{"/bundle/a/b/c.md", "a/b/c.md", true},
		// Non bundle-relative links.
		{"https://example.com/foo.md", "", false},
		{"http://example.com", "", false},
		{"mailto:foo@bar.com", "", false},
		{"tel:+15551234", "", false},
		{"#anchor", "", false},
		{"./relative.md", "", false},
		{"../parent.md", "", false},
		{"plain.md", "", false},
		{"", "", false},
		{"/bundle", "", false}, // missing trailing slash is not the bundle prefix
		{"bundle/foo.md", "", false},
	}
	for _, tc := range cases {
		rel, ok := ParseBundleRelative(tc.link)
		if rel != tc.wantRel || ok != tc.wantOK {
			t.Fatalf("ParseBundleRelative(%q) = (%q, %v), want (%q, %v)",
				tc.link, rel, ok, tc.wantRel, tc.wantOK)
		}
	}
}

func TestResolveRelativeSameDir(t *testing.T) {
	t.Parallel()
	got := Resolve("./gemma.md", "concepts/overview.md")
	want := "concepts/gemma.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveParentDir(t *testing.T) {
	t.Parallel()
	got := Resolve("../summary.md", "concepts/gemma.md")
	want := "summary.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveBareRelative(t *testing.T) {
	t.Parallel()
	// A bare file name resolves against the current file's directory.
	got := Resolve("sibling.md", "concepts/gemma.md")
	want := "concepts/sibling.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveMultipleParentDirs(t *testing.T) {
	t.Parallel()
	// "a/b" + "../.." walks all the way up to the bundle root.
	got := Resolve("../../root.md", "a/b/c.md")
	want := "root.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveParentDirNotBelowRoot(t *testing.T) {
	t.Parallel()
	// path.Clean preserves leading ".." when the parent walk overshoots the
	// root; for current="a/b/c.md" and three levels of ".." the result walks
	// past "a" and leaves one leading ".." on the result. Resolve does not
	// invent a virtual root, so callers that care should validate the result.
	got := Resolve("../../../root.md", "a/b/c.md")
	want := "../root.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveRootCurrentPath(t *testing.T) {
	t.Parallel()
	// A current path with no directory means base is ".".
	got := Resolve("./sibling.md", "root.md")
	want := "sibling.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveRejectsHTTPS(t *testing.T) {
	t.Parallel()
	link := "https://example.com/foo.md"
	if got := Resolve(link, "concepts/gemma.md"); got != link {
		t.Fatalf("Resolve = %q, want verbatim %q", got, link)
	}
}

func TestResolveRejectsAnchor(t *testing.T) {
	t.Parallel()
	link := "#section"
	if got := Resolve(link, "concepts/gemma.md"); got != link {
		t.Fatalf("Resolve = %q, want verbatim %q", got, link)
	}
}

func TestResolveRejectsMailto(t *testing.T) {
	t.Parallel()
	link := "mailto:foo@bar.com"
	if got := Resolve(link, "concepts/gemma.md"); got != link {
		t.Fatalf("Resolve = %q, want verbatim %q", got, link)
	}
}

func TestResolveRejectsFtpScheme(t *testing.T) {
	t.Parallel()
	link := "ftp://example.com/file.md"
	if got := Resolve(link, "a/b.md"); got != link {
		t.Fatalf("Resolve = %q, want verbatim %q", got, link)
	}
}

func TestResolveBundleRelativeUnchanged(t *testing.T) {
	t.Parallel()
	link := "/bundle/foo/bar.md"
	if got := Resolve(link, "concepts/gemma.md"); got != link {
		t.Fatalf("Resolve = %q, want verbatim %q", got, link)
	}
}

func TestResolveWindowsBackslashes(t *testing.T) {
	t.Parallel()
	// Windows-style backslashes in both rel and current should normalize.
	got := Resolve("..\\summary.md", "concepts\\gemma.md")
	want := "summary.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveWindowsBackslashesSameDir(t *testing.T) {
	t.Parallel()
	got := Resolve(".\\sibling.md", "concepts\\gemma.md")
	want := "concepts/sibling.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveEmptyRelPath(t *testing.T) {
	t.Parallel()
	if got := Resolve("", "a/b.md"); got != "." {
		t.Fatalf("Resolve(empty, ...) = %q, want \".\"", got)
	}
}

func TestResolveCleansDoubleSlashes(t *testing.T) {
	t.Parallel()
	// path.Clean collapses redundant separators introduced by mistake.
	got := Resolve("foo//bar.md", "concepts/gemma.md")
	want := "concepts/foo/bar.md"
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveNonSchemeColon(t *testing.T) {
	t.Parallel()
	// "a:b" parses as a scheme-prefixed URL (single-letter scheme is valid per
	// RFC 3986) and is therefore returned verbatim, not resolved relative to
	// the current document. This mirrors how a markdown renderer would treat
	// such a link as an external URL.
	link := "a:b"
	if got := Resolve(link, "concepts/gemma.md"); got != link {
		t.Fatalf("Resolve = %q, want verbatim %q", got, link)
	}
}
