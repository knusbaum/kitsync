package kitsync

import (
	"os"
	"path/filepath"
	"testing"
)

// ── ByNameAlpha sort test (via real files, since we can't construct os.DirEntry easily) ──
// newFSIndexIterator sorts via ByNameAlpha, so testing order indirectly tests it.

// ── fsIndexIterator (uses temp dirs) ─────────────────────────────────────

func createTestFile(t *testing.T, fullPath string) {
	t.Helper()
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(fullPath, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", fullPath, err)
	}
}

func TestFSIndexIteratorFlat(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "file1"))
	createTestFile(t, filepath.Join(dir, "file2"))
	createTestFile(t, filepath.Join(dir, "file3"))

	it, err := newFSIndexIterator(dir)
	if err != nil {
		t.Fatalf("newFSIndexIterator: %v", err)
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
	if len(ids) != 3 {
		t.Fatalf("got %d IDs, want 3: %v", len(ids), ids)
	}
	if ids[0] != "file1" || ids[1] != "file2" || ids[2] != "file3" {
		t.Errorf("IDs not in order: %v", ids)
	}
}

func TestFSIndexIteratorNested(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "a"))
	createTestFile(t, filepath.Join(dir, "sub", "b"))
	createTestFile(t, filepath.Join(dir, "sub", "c"))
	createTestFile(t, filepath.Join(dir, "sub", "deep", "d"))

	it, err := newFSIndexIterator(dir)
	if err != nil {
		t.Fatalf("newFSIndexIterator: %v", err)
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
	if len(ids) != 4 {
		t.Fatalf("got %d IDs, want 4: %v", len(ids), ids)
	}
}

func TestFSIndexIteratorSkipsExtensions(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "file1"))
	createTestFile(t, filepath.Join(dir, "file1.tags"))
	createTestFile(t, filepath.Join(dir, "file2"))

	it, err := newFSIndexIterator(dir)
	if err != nil {
		t.Fatalf("newFSIndexIterator: %v", err)
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
}

func TestFSIndexIteratorEmpty(t *testing.T) {
	dir := t.TempDir()
	it, err := newFSIndexIterator(dir)
	if err != nil {
		t.Fatalf("newFSIndexIterator: %v", err)
	}
	defer it.Close()

	id, err := it.Next()
	if err == nil {
		t.Errorf("expected error on empty dir, got id=%q", id)
	}
}

func TestFSIndexIteratorCloseIsNoop(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "a"))

	it, err := newFSIndexIterator(dir)
	if err != nil {
		t.Fatalf("newFSIndexIterator: %v", err)
	}
	if err := it.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Closing twice should not panic.
	if err := it.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestFSIndexIteratorIterateAfterClose(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "a"))

	it, err := newFSIndexIterator(dir)
	if err != nil {
		t.Fatalf("newFSIndexIterator: %v", err)
	}
	it.Close()

	id, err := it.Next()
	if err == nil {
		t.Errorf("expected error after close, got id=%q", id)
	}
}

