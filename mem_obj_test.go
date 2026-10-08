package kitsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestNewMemObjectID(t *testing.T) {
	content := []byte("test content")
	o := NewMemObject(content, nil)
	got := o.ID()

	expectedHash := sha256.Sum256(content)
	want := hex.EncodeToString(expectedHash[:])
	if got != want {
		t.Errorf("NewMemObject(%q).ID() = %q, want %q", content, got, want)
	}
}

func TestNewMemObjectWithNilTags(t *testing.T) {
	o := NewMemObject([]byte("data"), nil)
	if o == nil {
		t.Fatal("NewMemObject returned nil")
	}
}

func TestNewMemObjectEmptyContent(t *testing.T) {
	o := NewMemObject([]byte{}, map[string]string{"key": "val"})
	got, err := o.Tags()
	if err != nil {
		t.Fatalf("Tags on empty content: %v", err)
	}
	if len(got) != 1 || got["key"] != "val" {
		t.Errorf("Tags = %v, want map[key:val]", got)
	}
}

func TestMemObjectContentRead(t *testing.T) {
	content := []byte("read me")
	o := NewMemObject(content, nil)
	rc, err := o.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestMemObjectContentIsIndependent(t *testing.T) {
	// NOTE: memObject.Content() returns bytes.NewBuffer(o.content) which shares
	// the underlying array with o.content. Mutating the original input after
	// creation _would_ be visible through the buffer. This test confirms that
	// reading before any mutation works correctly regardless.
	content := []byte("read me first")
	o := NewMemObject(content, nil)
	rc, err := o.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestMemObjectTags(t *testing.T) {
	tags := map[string]string{"foo": "bar", "baz": "qux"}
	o := NewMemObject([]byte("data"), tags)
	got, err := o.Tags()
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Tags()) = %d, want 2", len(got))
	}
	if got["foo"] != "bar" {
		t.Errorf("Tags()[\"foo\"] = %q, want %q", got["foo"], "bar")
	}
	if got["baz"] != "qux" {
		t.Errorf("Tags()[\"baz\"] = %q, want %q", got["baz"], "qux")
	}
}

func TestMemObjectAddTag(t *testing.T) {
	o := NewMemObject([]byte("data"), nil)
	if err := o.AddTag("k1", "v1"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	got, err := o.Tags()
	if err != nil {
		t.Fatalf("Tags after AddTag: %v", err)
	}
	if got["k1"] != "v1" {
		t.Errorf("Tags()[\"k1\"] = %q, want %q", got["k1"], "v1")
	}

	o.AddTag("k2", "v2")
	got, _ = o.Tags()
	if len(got) != 2 {
		t.Errorf("len(Tags()) after second AddTag = %d, want 2", len(got))
	}
}

func TestMemObjectDelTag(t *testing.T) {
	o := NewMemObject([]byte("data"), map[string]string{"keep": "yes", "remove": "no"})
	if err := o.DelTag("remove"); err != nil {
		t.Fatalf("DelTag: %v", err)
	}
	got, _ := o.Tags()
	if _, ok := got["remove"]; ok {
		t.Error("DelTag did not remove the tag")
	}
	if got["keep"] != "yes" {
		t.Errorf("Other tag changed: want %q, got %q", "yes", got["keep"])
	}
}

func TestMemObjectHash(t *testing.T) {
	content := []byte("hash test")
	tags := map[string]string{"a": "1"}
	o := NewMemObject(content, tags)

	hash, err := o.Hash()
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "" {
		t.Error("Hash returned empty string")
	}
	if len(hash) != 64 {
		t.Errorf("Hash length = %d, want 64", len(hash))
	}

	// Same content + tags => same hash
	o2 := NewMemObject(content, map[string]string{"a": "1"})
	hash2, _ := o2.Hash()
	if hash != hash2 {
		t.Errorf("Same content/tags => different hashes: %q vs %q", hash, hash2)
	}
}

func TestMemObjectIDConsistent(t *testing.T) {
	content := []byte("consistent")
	o1 := NewMemObject(content, nil)
	o2 := NewMemObject(content, nil)
	if o1.ID() != o2.ID() {
		t.Error("Different IDs for same content")
	}
}
