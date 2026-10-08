package kitsync

import (
	"io"
	"os"
	"testing"
)

// ── pathForHash ──────────────────────────────────────────────────────────

func TestPathForHashValid64(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got, ok := pathForHash(hash)
	if !ok {
		t.Fatal("pathForHash should succeed for 64-char hex")
	}
	// Format: first 4 / next 4 / next 4 / full hash
	want := "0123/4567/89ab/" + hash
	if got != want {
		t.Errorf("pathForHash(%q) = %q, want %q", hash, got, want)
	}
}

func TestPathForHashTruncated(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef" // 32 chars
	got, ok := pathForHash(hash)
	if !ok {
		t.Fatal("pathForHash should succeed for 32-char hex")
	}
	want := "0123/4567/89ab/" + hash
	if got != want {
		t.Errorf("pathForHash(%q) = %q, want %q", hash, got, want)
	}
}

func TestPathForHashShort(t *testing.T) {
	for _, h := range []string{"", "abc", "0123456789a"} { // 11 chars
		got, ok := pathForHash(h)
		if ok {
			t.Errorf("pathForHash(%q) should return ok=false for len=%d", h, len(h))
		}
		if got != "" {
			t.Errorf("pathForHash(%q) = %q, want \"\"", h, got)
		}
	}
}

func TestPathForHashExactly12Chars(t *testing.T) {
	hash := "0123456789ab" // exactly 12
	got, ok := pathForHash(hash)
	if !ok {
		t.Fatal("pathForHash should succeed for exactly 12 chars")
	}
	want := "0123/4567/89ab/0123456789ab"
	if got != want {
		t.Errorf("pathForHash(%q) = %q, want %q", hash, got, want)
	}
}

func TestPathForHashNonHexLengthOK(t *testing.T) {
	hash := "gggggggggggg" // 12 chars, non-hex — pathForHash doesn't validate hex
	got, ok := pathForHash(hash)
	if !ok {
		t.Error("pathForHash accepts non-hex as long as length >= 12")
	}
	want := "gggg/gggg/gggg/gggggggggggg"
	if got != want {
		t.Errorf("pathForHash(%q) = %q, want %q", hash, got, want)
	}
}

// ── validHash ────────────────────────────────────────────────────────────

func TestValidHashTrue64(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if !validHash(hash) {
		t.Error("validHash should be true for a valid 64-char hex string")
	}
}

func TestValidHashFalseShort(t *testing.T) {
	for _, h := range []string{"", "0123456789abcde"} {
		if validHash(h) {
			t.Errorf("validHash(%q) = true, want false for len=%d", h, len(h))
		}
	}
}

func TestValidHashFalseInvalidChars(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef01234567gggg"
	if validHash(hash) {
		t.Error("validHash should be false for hex string with non-hex chars")
	}
}

func TestValidHashUpperCase(t *testing.T) {
	hash := "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
	if !validHash(hash) {
		t.Error("validHash should be true for uppercase hex")
	}
}

// ── calculateHash ────────────────────────────────────────────────────────

func TestCalculateHashDeterministic(t *testing.T) {
	h1 := calculateHash("abc", map[string]string{"x": "1"})
	h2 := calculateHash("abc", map[string]string{"x": "1"})
	if h1 != h2 {
		t.Errorf("calculateHash not deterministic: %q vs %q", h1, h2)
	}
}

func TestCalculateHashTagOrderIndependent(t *testing.T) {
	m1 := map[string]string{"b": "2", "a": "1"}
	m2 := map[string]string{"a": "1", "b": "2"}
	h1 := calculateHash("abc", m1)
	h2 := calculateHash("abc", m2)
	if h1 != h2 {
		t.Errorf("calculateHash depends on map iteration order: %q vs %q", h1, h2)
	}
}

func TestCalculateHashDifferentContent(t *testing.T) {
	h1 := calculateHash("aaa", nil)
	h2 := calculateHash("bbb", nil)
	if h1 == h2 {
		t.Error("calculateHash should differ for different IDs")
	}
}

func TestCalculateHashDifferentTagKeys(t *testing.T) {
	h1 := calculateHash("abc", map[string]string{"a": "v"})
	h2 := calculateHash("abc", map[string]string{"b": "v"})
	if h1 == h2 {
		t.Error("calculateHash should differ for different tag keys")
	}
}

func TestCalculateHashSameKeyDifferentValue(t *testing.T) {
	// calculateHash only hashes ID and key names, NOT values.
	h1 := calculateHash("abc", map[string]string{"k": "v1"})
	h2 := calculateHash("abc", map[string]string{"k": "v2"})
	if h1 != h2 {
		t.Errorf("calculateHash with same keys but different values should match: %q vs %q", h1, h2)
	}
}

func TestCalculateHashNilVsEmptyTags(t *testing.T) {
	hNil := calculateHash("abc", nil)
	hEmpty := calculateHash("abc", map[string]string{})
	if hNil != hEmpty {
		t.Errorf("nil vs empty tags produce different hashes: %q vs %q", hNil, hEmpty)
	}
}

// ── fsStorage (integration-style with temp dirs) ─────────────────────────

func newTestFSStorage(t *testing.T) (*fsStorage, func()) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewFSStorage(dir)
	if err != nil {
		t.Fatalf("NewFSStorage: %v", err)
	}
	return s, func() { os.RemoveAll(dir) }
}

func TestFSStoragePutAndGet(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	o := NewMemObject([]byte("put this"), map[string]string{"env": "test"})
	if err := s.Put(o); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok := s.Get(o.ID())
	if !ok {
		t.Fatal("Get returned not present")
	}
	if got.ID() != o.ID() {
		t.Errorf("ID = %q, want %q", got.ID(), o.ID())
	}
}

func TestFSStoragePresent(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	content := []byte("data")
	o := NewMemObject(content, nil)

	if s.Present(o.ID()) {
		t.Error("Present before Put should be false")
	}

	if err := s.Put(o); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !s.Present(o.ID()) {
		t.Error("Present after Put should be true")
	}
}

func TestFSStorageDelete(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	o := NewMemObject([]byte("delete me"), nil)
	if err := s.Put(o); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !s.Present(o.ID()) {
		t.Fatal("not present before delete")
	}

	if err := s.Delete(o.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, ok := s.Get(o.ID()); ok {
		t.Error("Get after Delete should return not present")
	}
	if s.Present(o.ID()) {
		t.Error("Present after Delete should be false")
	}
}

func TestFSStoragePutIdempotent(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	o := NewMemObject([]byte("same data"), nil)
	if err := s.Put(o); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	if err := s.Put(o); err != nil {
		t.Fatalf("second Put (should be no-op): %v", err)
	}

	got, ok := s.Get(o.ID())
	if !ok {
		t.Fatal("missing after idempotent put")
	}
	rc, err := got.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(content) != "same data" {
		t.Errorf("content = %q, want %q", content, "same data")
	}
}

func TestFSStorageIter(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	o1 := NewMemObject([]byte("first"), nil)
	o2 := NewMemObject([]byte("second"), nil)
	if err := s.Put(o1); err != nil {
		t.Fatalf("Put first: %v", err)
	}
	if err := s.Put(o2); err != nil {
		t.Fatalf("Put second: %v", err)
	}

	it, err := s.Iter()
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	defer it.Close()

	var ids []string
	for {
		id, err := it.Next()
		if err != nil {
			break
		}
		ids = append(ids, id)
	}
	if len(ids) != 2 {
		t.Errorf("got %d IDs, want 2: %v", len(ids), ids)
	}

	// Should be sorted alphabetically.
	if len(ids) == 2 && ids[0] > ids[1] {
		t.Errorf("IDs not sorted: %q vs %q", ids[0], ids[1])
	}
}

func TestFSStorageGetNonexistent(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	_, ok := s.Get("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if ok {
		t.Error("Get nonexistent should return not present")
	}
}

func TestFSStoragePresentWithBadID(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	// Invalid hash (too short, not hex) → Present returns false
	if s.Present("short") {
		t.Error("Present with bad ID should return false")
	}
}

func TestFSStorageStringer(t *testing.T) {
	s := &fsStorage{root: "/tmp/kitsync_test"}
	got := s.String()
	wantPrefix := "Local(/tmp/kitsync_test)"
	if got != wantPrefix {
		t.Errorf("String() = %q, want %q", got, wantPrefix)
	}
}

func TestFSStorageCheckpoint(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFSStorage(dir)
	if err != nil {
		t.Fatalf("NewFSStorage: %v", err)
	}
	defer os.RemoveAll(dir)

	// Before set checkpoint, should return 0.
	if s.Checkpoint() != 0 {
		t.Errorf("Checkpoint = %d, want 0 before SetCheckpoint", s.Checkpoint())
	}

	s.SetCheckpoint(42)
	got := s.Checkpoint()
	if got != 42 {
		t.Errorf("Checkpoint = %d, want 42", got)
	}
}

func TestFSStorageDeleteNonexistent(t *testing.T) {
	s, cleanup := newTestFSStorage(t)
	defer cleanup()

	err := s.Delete("aaaa") // invalid hash format → error from pathForHash
	if err == nil {
		t.Error("Delete with bad ID should return an error")
	}
}
