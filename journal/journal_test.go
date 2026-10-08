package journal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testEvent struct {
	Action string
	Value  int
}

func newTestJournal(t *testing.T, size int) *Journal[testEvent] {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "journal")
	j, err := NewJournal[testEvent](path, size, time.Millisecond)
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	return j
}

func TestJournalAddAndGet(t *testing.T) {
	j := newTestJournal(t, 10)
	defer j.Close()

	idx, err := j.Add(testEvent{"create", 42})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if idx != 1 {
		t.Fatalf("expected index 1, got %d", idx)
	}

	ev, err := j.Get(idx)
	if err != nil {
		t.Fatalf("Get(%d): %v", idx, err)
	}
	if ev.Action != "create" || ev.Value != 42 {
		t.Fatalf("expected create/42, got %+v", ev)
	}
}

func TestJournalSequentialAdd(t *testing.T) {
	j := newTestJournal(t, 10)
	defer j.Close()

	want := []testEvent{
		{"create", 1},
		{"update", 2},
		{"delete", 3},
	}
	var indices []uint64
	for i, v := range want {
		idx, err := j.Add(v)
		if err != nil {
			t.Fatalf("Add[%d]: %v", i, err)
		}
		indices = append(indices, idx)
	}

	for i, gotIdx := range indices {
		ev, err := j.Get(gotIdx)
		if err != nil {
			t.Fatalf("Get(%d): %v", gotIdx, err)
		}
		if ev != want[i] {
			t.Errorf("Get(%d) = %+v, want %+v", gotIdx, ev, want[i])
		}
	}
}

func TestJournalLatest(t *testing.T) {
	j := newTestJournal(t, 10)
	defer j.Close()

	if j.Latest() != 0 {
		t.Fatalf("empty journal Latest = %d, want 0", j.Latest())
	}

	for i := 0; i < 5; i++ {
		j.Add(testEvent{"x", i})
		if j.Latest() != uint64(i+1) {
			t.Fatalf("after %d adds, Latest = %d, want %d", i+1, j.Latest(), i+1)
		}
	}
}

func TestJournalWraparound(t *testing.T) {
	const size = 5
	j := newTestJournal(t, size)
	defer j.Close()

	// Write exactly size entries — they are all readable.
	for i := 0; i < size; i++ {
		j.Add(testEvent{"seq", i})
	}
	// All entries still valid before wraparound.
	for i := uint64(1); i <= uint64(size); i++ {
		ev, err := j.Get(i)
		if err != nil {
			t.Fatalf("Get(%d) before wraparound: %v", i, err)
		}
		if ev.Value != int(i-1) {
			t.Errorf("Get(%d).Value = %d, want %d", i, ev.Value, i-1)
		}
	}

	// Write one more — triggers wraparound (slot 0 reused for index=size+1).
	j.Add(testEvent{"wrap", 99})

	// Entry 1 is gone (its slot was overwritten).
	_, err := j.Get(1)
	if err != ErrNotPresent {
		t.Fatalf("Get(1) after wraparound: expected ErrNotPresent, got %v", err)
	}
	// The new entry at index=size+1 must be readable.
	ev, err := j.Get(uint64(size + 1))
	if err != nil {
		t.Fatalf("Get(%d): %v", size+1, err)
	}
	if ev.Value != 99 {
		t.Errorf("Get(%d).Value = %d, want 99", size+1, ev.Value)
	}

	// The other original entries remain readable because their slots haven't been reused yet.
	for i := uint64(2); i <= uint64(size); i++ {
		ev, err := j.Get(i)
		if err != nil {
			t.Fatalf("Get(%d) after wraparound: %v", i, err)
		}
		if ev.Value != int(i-1) {
			t.Errorf("Get(%d).Value = %d, want %d", i, ev.Value, i-1)
		}
	}
}

func TestJournalOverwriteOldEntry(t *testing.T) {
	const size = 3
	j := newTestJournal(t, size)
	defer j.Close()

	// Fill buffer: indices 1, 2, 3 in slots 0, 1, 2.
	for i := 1; i <= 3; i++ {
		j.Add(testEvent{"fill", i})
	}

	// Add two more: index=4 overwrites slot 0 (was index=1),
	// index=5 overwrites slot 1 (was index=2).
	j.Add(testEvent{"over", 4})
	j.Add(testEvent{"over", 5})

	// Index 1 and 2 gone.
	for _, idx := range []uint64{1, 2} {
		_, err := j.Get(idx)
		if err != ErrNotPresent {
			t.Errorf("Get(%d): expected ErrNotPresent, got %v", idx, err)
		}
	}

	// Index 3 is still there (slot 2).
	ev, err := j.Get(3)
	if err != nil {
		t.Fatalf("Get(3): %v", err)
	}
	if ev.Value != 3 {
		t.Errorf("Get(3).Value = %d, want 3", ev.Value)
	}
}

func TestJournalZeroIndexNotPresent(t *testing.T) {
	j := newTestJournal(t, 5)
	defer j.Close()

	_, err := j.Get(0)
	if err != ErrNotPresent {
		t.Fatalf("Get(0): expected ErrNotPresent, got %v", err)
	}
}

func TestJournalClosedRejectsWrites(t *testing.T) {
	j := newTestJournal(t, 5)
	j.Close()

	_, err := j.Add(testEvent{"nope", 0})
	if err != ErrClosed {
		t.Fatalf("Add after close: expected ErrClosed, got %v", err)
	}
}

func TestJournalCloseFlushes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal")
	j, err := NewJournal[testEvent](path, 10, time.Millisecond)
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}

	for i := 0; i < 4; i++ {
		j.Add(testEvent{"persist", i})
	}
	j.Close()

	// Verify the journal file exists on disk.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("journal file not found after Close")
	}

	// Open and verify entries survive.
	j2, err := OpenJournal[testEvent](path)
	if err != nil {
		t.Fatalf("OpenJournal: %v", err)
	}
	defer j2.Close()

	for i := uint64(1); i <= 4; i++ {
		ev, err := j2.Get(i)
		if err != nil {
			t.Fatalf("Get(%d) from reopened journal: %v", i, err)
		}
		if ev.Value != int(i-1) {
			t.Errorf("Get(%d).Value = %d, want %d", i, ev.Value, i-1)
		}
	}
}

func TestJournalReset(t *testing.T) {
	j := newTestJournal(t, 5)
	defer j.Close()

	for i := 0; i < 3; i++ {
		j.Add(testEvent{"before", i})
	}

	j.Reset()

	// Old entries gone.
	for i := uint64(1); i <= 3; i++ {
		_, err := j.Get(i)
		if err != ErrNotPresent {
			t.Errorf("Get(%d) after reset: expected ErrNotPresent, got %v", i, err)
		}
	}

	// Can add fresh entries starting at index 1.
	idx, err := j.Add(testEvent{"after", 0})
	if err != nil {
		t.Fatalf("Add after reset: %v", err)
	}
	if idx != 1 {
		t.Errorf("index after reset = %d, want 1", idx)
	}
}

func TestJournalDoubleClose(t *testing.T) {
	j := newTestJournal(t, 5)
	j.Add(testEvent{"x", 1})
	j.Close()
	// Should not panic.
	j.Close()
}
