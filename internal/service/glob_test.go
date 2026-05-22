package service

import "testing"

func TestMatchGlob_ExactPath(t *testing.T) {
	if !MatchGlob("/Documents/Report.pdf", "/Documents/Report.pdf") {
		t.Error("exact path should match itself")
	}
}

func TestMatchGlob_ExactPath_NotMatch(t *testing.T) {
	if MatchGlob("/Documents/Report.pdf", "/Documents/Other.pdf") {
		t.Error("different paths should not match")
	}
}

func TestMatchGlob_SingleWildcard(t *testing.T) {
	if !MatchGlob("/Documents/*", "/Documents/file.pdf") {
		t.Error("/* should match direct child")
	}
}

func TestMatchGlob_SingleWildcard_NotMatchDeep(t *testing.T) {
	if MatchGlob("/Documents/*", "/Documents/sub/file.pdf") {
		t.Error("/* should not match nested paths")
	}
}

func TestMatchGlob_DoubleWildcard(t *testing.T) {
	if !MatchGlob("/Documents/**", "/Documents/sub/file.pdf") {
		t.Error("/** should match nested paths")
	}
	if !MatchGlob("/Documents/**", "/Documents/file.pdf") {
		t.Error("/** should match direct child")
	}
	if !MatchGlob("/Documents/**", "/Documents/a/b/c/file.pdf") {
		t.Error("/** should match deeply nested paths")
	}
}

func TestMatchGlob_DoubleWildcard_NotMatchOutside(t *testing.T) {
	if MatchGlob("/Documents/**", "/Downloads/file.pdf") {
		t.Error("/** should not match outside prefix")
	}
}

func TestMatchGlob_ExtensionPattern(t *testing.T) {
	if !MatchGlob("/Documents/**/*.pdf", "/Documents/a/report.pdf") {
		t.Error("/**/*.pdf should match pdf in subfolder")
	}
	if MatchGlob("/Documents/**/*.pdf", "/Documents/a/report.doc") {
		t.Error("/**/*.pdf should not match doc")
	}
}

func TestMatchGlob_RootWildcard(t *testing.T) {
	if !MatchGlob("/**", "/Documents/file.pdf") {
		t.Error("/** should match everything")
	}
	if !MatchGlob("/**", "/a/b/c") {
		t.Error("/** should match deep paths")
	}
}

func TestMatchGlob_RootExtension(t *testing.T) {
	if !MatchGlob("/**/*.txt", "/Documents/readme.txt") {
		t.Error("/**/*.txt should match txt at any depth")
	}
	if !MatchGlob("/**/*.txt", "/a/b/c/file.txt") {
		t.Error("/**/*.txt should match deeply nested txt")
	}
	if MatchGlob("/**/*.txt", "/a/b/c/file.pdf") {
		t.Error("/**/*.txt should not match pdf")
	}
}

func TestMatchGlob_MixedPattern(t *testing.T) {
	if !MatchGlob("/Docs/*/report-*.pdf", "/Docs/projectA/report-v1.pdf") {
		t.Error("mixed pattern should match")
	}
	if MatchGlob("/Docs/*/report-*.pdf", "/Docs/a/b/report-v1.pdf") {
		t.Error("mixed pattern should not match double nested")
	}
}

func TestMatchGlob_NoLeadingSlash(t *testing.T) {
	if MatchGlob("/**", "no-leading-slash") {
		t.Error("path without leading / should not match")
	}
}

func TestMatchGlob_EmptyPattern(t *testing.T) {
	if MatchGlob("", "/some/path") {
		t.Error("empty pattern should not match")
	}
}

func TestMatchGlob_ExtensionOnlyFilter(t *testing.T) {
	if !MatchGlob("/**/*.txt", "/notes.txt") {
		t.Error("/**/*.txt should match root level txt")
	}
}

func TestMatchGlob_DoubleWildcardMid(t *testing.T) {
	if !MatchGlob("/a/**/c", "/a/b/c") {
		t.Error("/a/**/c should match /a/b/c")
	}
	if !MatchGlob("/a/**/c", "/a/x/y/z/c") {
		t.Error("/a/**/c should match /a/x/y/z/c")
	}
	if !MatchGlob("/a/**/c", "/a/c") {
		t.Error("/a/**/c should match /a/c (zero segments)")
	}
}
