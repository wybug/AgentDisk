package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempLocalStorage(t *testing.T) *LocalStorage {
	t.Helper()
	dir := t.TempDir()
	return NewLocalStorage(dir, "test-hmac-secret")
}

func TestLocalStorage_UploadDownload(t *testing.T) {
	s := tempLocalStorage(t)
	ctx := context.Background()

	content := []byte("hello world")
	if err := s.Upload(ctx, "disk/user_001/test.txt", bytes.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	r, err := s.Download(ctx, "disk/user_001/test.txt")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestLocalStorage_Copy(t *testing.T) {
	s := tempLocalStorage(t)
	ctx := context.Background()

	content := []byte("copy me")
	if err := s.Upload(ctx, "src.txt", bytes.NewReader(content), int64(len(content)), ""); err != nil {
		t.Fatalf("Upload src: %v", err)
	}
	if err := s.Copy(ctx, "src.txt", "dst.txt"); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	r, err := s.Download(ctx, "dst.txt")
	if err != nil {
		t.Fatalf("Download dst: %v", err)
	}
	defer r.Close()

	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, content) {
		t.Fatalf("copy content mismatch: got %q, want %q", got, content)
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	s := tempLocalStorage(t)
	ctx := context.Background()

	if err := s.Upload(ctx, "del.txt", bytes.NewReader([]byte("x")), 1, ""); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if err := s.Delete(ctx, "del.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Download(ctx, "del.txt"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestLocalStorage_EnsureBucket(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "storage-test-ensure")
	defer os.RemoveAll(dir)

	s := NewLocalStorage(dir, "secret")
	if err := s.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("root dir not created")
	}
}

func TestLocalStorage_PresignedGetURL(t *testing.T) {
	s := tempLocalStorage(t)
	url, err := s.PresignedGetURL(context.Background(), "disk/user_001/file.txt", time.Hour)
	if err != nil {
		t.Fatalf("PresignedGetURL: %v", err)
	}
	if url == "" {
		t.Fatal("empty URL")
	}
}

func TestLocalStorage_VerifySignature_Valid(t *testing.T) {
	s := tempLocalStorage(t)
	exp := time.Now().Add(time.Hour).Unix()
	sig := s.sign("key.txt", exp)
	if !s.VerifySignature("key.txt", exp, sig) {
		t.Fatal("signature should be valid")
	}
}

func TestLocalStorage_VerifySignature_Expired(t *testing.T) {
	s := tempLocalStorage(t)
	exp := time.Now().Add(-time.Hour).Unix()
	sig := s.sign("key.txt", exp)
	// VerifySignature only checks hmac match, expiration is checked by handler
	if !s.VerifySignature("key.txt", exp, sig) {
		t.Fatal("signature should match even if expired (handler checks expiry)")
	}
}

func TestLocalStorage_VerifySignature_Tampered(t *testing.T) {
	s := tempLocalStorage(t)
	exp := time.Now().Add(time.Hour).Unix()
	if s.VerifySignature("key.txt", exp, "tampered-sig") {
		t.Fatal("tampered signature should not match")
	}
}

func TestLocalStorage_VerifySignature_WrongKey(t *testing.T) {
	s := tempLocalStorage(t)
	exp := time.Now().Add(time.Hour).Unix()
	sig := s.sign("key.txt", exp)
	if s.VerifySignature("other.txt", exp, sig) {
		t.Fatal("wrong key should not match")
	}
}
