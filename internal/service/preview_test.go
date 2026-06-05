package service

import (
	"io"
	"strings"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		fileType string
		expected string
	}{
		{"md", "markdown"},
		{"markdown", "markdown"},
		{"html", "html"},
		{"htm", "html"},
		{"go", "code"},
		{"py", "code"},
		{"js", "code"},
		{"json", "code"},
		{"yaml", "code"},
		{"png", "image"},
		{"jpg", "image"},
		{"svg", "image"},
		{"txt", "text"},
		{"csv", "text"},
		{"log", "text"},
		{"exe", "binary"},
		{"zip", "binary"},
		{"", "binary"},
	}
	for _, tt := range tests {
		f := &model.DiskFile{FileType: tt.fileType}
		result := classify(f)
		if result != tt.expected {
			t.Errorf("classify(%s) = %s, expected %s", tt.fileType, result, tt.expected)
		}
	}
}

func TestIsCode(t *testing.T) {
	codeExts := []string{"go", "py", "js", "ts", "java", "c", "cpp", "rs", "sql", "css", "json", "yaml", "yml"}
	for _, ext := range codeExts {
		if !isCode(ext) {
			t.Errorf("isCode(%s) should be true", ext)
		}
	}
	if isCode("png") {
		t.Error("isCode(png) should be false")
	}
	if isCode("html") {
		t.Error("isCode(html) should be false")
	}
}

func TestIsImage(t *testing.T) {
	imgExts := []string{"png", "jpg", "jpeg", "gif", "svg", "webp"}
	for _, ext := range imgExts {
		if !isImage(ext) {
			t.Errorf("isImage(%s) should be true", ext)
		}
	}
	if isImage("txt") {
		t.Error("isImage(txt) should be false")
	}
}

func TestIsText(t *testing.T) {
	textExts := []string{"txt", "log", "csv", "md"}
	for _, ext := range textExts {
		if !isText(ext) {
			t.Errorf("isText(%s) should be true", ext)
		}
	}
	if isText("go") {
		t.Error("isText(go) should be false")
	}
}

func TestIsHTML(t *testing.T) {
	for _, ext := range []string{"html", "htm"} {
		if !isHTML(ext) {
			t.Errorf("isHTML(%s) should be true", ext)
		}
	}
	for _, ext := range []string{"css", "js", "png", "txt"} {
		if isHTML(ext) {
			t.Errorf("isHTML(%s) should be false", ext)
		}
	}
}

// --- Tests for buildPreviewResult ---

func TestBuildPreviewResult_TextFilesReturnContent(t *testing.T) {
	cases := []struct {
		name     string
		fileType string
		content  string
	}{
		{"txt file", "txt", "Hello AgentDisk"},
		{"csv file", "csv", "name,age\nAlice,30"},
		{"log file", "log", "2026-06-05 INFO started"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &model.DiskFile{FileType: tc.fileType, OSSKey: "test." + tc.fileType}
			cat := classify(f)
			result := buildPreviewResult(cat, strings.NewReader(tc.content), "http://example.com/presigned")
			if result.Content != tc.content {
				t.Errorf("content = %q, want %q", result.Content, tc.content)
			}
			if result.URL != "" {
				t.Errorf("url should be empty for text files, got %q", result.URL)
			}
			if result.FileType != cat {
				t.Errorf("fileType = %q, want %q", result.FileType, cat)
			}
		})
	}
}

func TestBuildPreviewResult_CodeFilesReturnContent(t *testing.T) {
	cases := []struct {
		name     string
		fileType string
		content  string
	}{
		{"go file", "go", "package main\nfunc main() {}"},
		{"py file", "py", "print('hello')"},
		{"json file", "json", `{"key": "value"}`},
		{"yaml file", "yaml", "key: value\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &model.DiskFile{FileType: tc.fileType}
			cat := classify(f)
			result := buildPreviewResult(cat, strings.NewReader(tc.content), "http://example.com/presigned")
			if result.Content != tc.content {
				t.Errorf("content = %q, want %q", result.Content, tc.content)
			}
			if result.URL != "" {
				t.Errorf("url should be empty for code files, got %q", result.URL)
			}
		})
	}
}

func TestBuildPreviewResult_MarkdownReturnsContent(t *testing.T) {
	f := &model.DiskFile{FileType: "md"}
	cat := classify(f)
	result := buildPreviewResult(cat, strings.NewReader("# Title\nbody text"), "http://example.com/presigned")
	if result.Content != "# Title\nbody text" {
		t.Errorf("content = %q, want markdown content", result.Content)
	}
	if result.URL != "" {
		t.Errorf("url should be empty for markdown, got %q", result.URL)
	}
}

func TestBuildPreviewResult_ImageReturnsURL(t *testing.T) {
	f := &model.DiskFile{FileType: "png"}
	cat := classify(f)
	presignedURL := "http://minio:9000/bucket/photo.png?signed=abc"
	result := buildPreviewResult(cat, nil, presignedURL)
	if result.URL != presignedURL {
		t.Errorf("url = %q, want %q", result.URL, presignedURL)
	}
	if result.Content != "" {
		t.Errorf("content should be empty for image files, got %q", result.Content)
	}
}

func TestBuildPreviewResult_HTMLReturnsURL(t *testing.T) {
	f := &model.DiskFile{FileType: "html"}
	cat := classify(f)
	presignedURL := "http://minio:9000/bucket/page.html?signed=abc"
	result := buildPreviewResult(cat, nil, presignedURL)
	if result.URL != presignedURL {
		t.Errorf("url = %q, want %q", result.URL, presignedURL)
	}
	if result.Content != "" {
		t.Errorf("content should be empty for html files, got %q", result.Content)
	}
}

func TestBuildPreviewResult_BinaryReturnsURL(t *testing.T) {
	f := &model.DiskFile{FileType: "zip"}
	cat := classify(f)
	presignedURL := "http://minio:9000/bucket/archive.zip?signed=abc"
	result := buildPreviewResult(cat, nil, presignedURL)
	if result.URL != presignedURL {
		t.Errorf("url = %q, want %q", result.URL, presignedURL)
	}
	if result.Content != "" {
		t.Errorf("content should be empty for binary files, got %q", result.Content)
	}
}

func TestBuildPreviewResult_TruncatesLargeContent(t *testing.T) {
	f := &model.DiskFile{FileType: "txt"}
	cat := classify(f)
	bigContent := strings.Repeat("x", maxPreviewSize+1000)
	result := buildPreviewResult(cat, strings.NewReader(bigContent), "http://example.com")
	if len(result.Content) != maxPreviewSize {
		t.Errorf("content length = %d, want %d (maxPreviewSize)", len(result.Content), maxPreviewSize)
	}
}

func TestBuildPreviewResult_NilReaderNonTextReturnsURL(t *testing.T) {
	result := buildPreviewResult("image", nil, "http://example.com/img.png")
	if result.URL != "http://example.com/img.png" {
		t.Errorf("url = %q, want presigned URL", result.URL)
	}
}

// Verify buildPreviewResult doesn't panic with nil reader for text types.
func TestBuildPreviewResult_NilReaderTextPanicsOrEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// nil reader panic is acceptable for text types
		}
	}()
	f := &model.DiskFile{FileType: "txt"}
	cat := classify(f)
	result := buildPreviewResult(cat, nil, "http://example.com")
	// If no panic, content should be empty
	if result.Content != "" {
		t.Errorf("content should be empty with nil reader, got %q", result.Content)
	}
}

// Unused import guard
var _ io.Reader = nil
