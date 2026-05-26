package service

import (
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
