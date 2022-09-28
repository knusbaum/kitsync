package journal

import (
	"encoding/gob"
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

var (
	ErrClosed     = errors.New("The Journal is already closed.")
	ErrNotPresent = errors.New("The entry at that index is not present in the journal.")
)

type entry[T any] struct {
	I uint64
	E T
}

// Journal is a circular buffer to which entries can be added. Each entry has an index, and Journal
// users can look up an entry by index. Indexes are sequential integers. The entry after
// entry(index) is entry(index+1).
//
// Journal uses a uint64 as the index, which is big enough to last for hundreds of years of
// constant writes, which I don't expect to happen.
//
// Entries of type T will be written to persistent storage using encoding/gob, and so only exported
// members of T are preserved.
//
// Journal is written to persistent storage periodically and on Close(). The period is
// configurable. Defering Close() is recommended, as that ensures safe cleanup on panics. While
// this will cover many failure modes, it won't cover others such as power/hardware failure,
// SIGKILL, or other termination that does not permit Go to execute deferred logic. In such cases,
// the journal entries since the last periodic write are lost.
//
// Note:
//   I intend to improve the durability characteristics of this journal later, so that entries are
//   not lost. The use-case for this journal is a read-heavy archive, with occasional writes, so this
//   simple journal mechanism is good enough for a first pass.
type Journal[T any] struct {
	cbuf   []entry[T]    // cbuf is the circular buffer of entries of T
	index  uint64        // index is the location where the NEXT addition will go.
	period time.Duration // period determines how often the journal is persisted to disk.

	shutdown chan struct{}
	fname    string
	l        sync.RWMutex
	wg       sync.WaitGroup
	dirty    bool
}

// gJournal is a private version of Journal with exported fields, so that encoding/gob can actually
// read/write the thing, but without exposing the internals of the exported Journal type.
type gJournal[T any] struct {
	Cbuf   []entry[T]
	Index  uint64
	Period time.Duration
}

func NewJournal[T any](file string, size int, period time.Duration) (*Journal[T], error) {
	j := &Journal[T]{
		cbuf:   make([]entry[T], size),
		index:  1,
		period: period,
		//
		shutdown: make(chan struct{}),
		fname:    file,
	}

	j.dirty = true
	err := j.Flush()
	if err != nil {
		return nil, err
	}
	go j.worker()
	return j, nil
}

func OpenJournal[T any](file string) (*Journal[T], error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var gj gJournal[T]
	d := gob.NewDecoder(f)
	// Intentionally using positional literal to ensure we don't forget to add fields here.
	err = d.Decode(&gj)
	if err != nil {
		return nil, err
	}

	j := &Journal[T]{
		cbuf:   gj.Cbuf,
		index:  gj.Index,
		period: gj.Period,
		//
		shutdown: make(chan struct{}),
		fname:    file,
	}
	go j.worker()
	return j, nil
}

func (j *Journal[T]) Reset() {
	j.l.Lock()
	defer j.l.Unlock()
	if !j.isOpen() {
		return
	}
	log.Printf("Journal reset.\n")
	j.cbuf = make([]entry[T], len(j.cbuf))
	j.index = 1
	j.dirty = true
	j.lockedFlush()
}

func (j *Journal[T]) writeNew(fname string) error {
	// We're writing a temporary file first to avoid trashing the good journal in case of error.
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer f.Close()

	e := gob.NewEncoder(f)
	// Intentionally using positional literal to ensure we don't forget to add fields here.
	err = e.Encode(gJournal[T]{j.cbuf, j.index, j.period})
	if err != nil {
		return err
	}
	return nil
}

func (j *Journal[T]) isOpen() bool {
	select {
	case <-j.shutdown:
		return false
	default:
		return true
	}
}

func (j *Journal[T]) Flush() error {
	j.l.Lock()
	defer j.l.Unlock()
	return j.lockedFlush()
}

func (j *Journal[T]) lockedFlush() error {
	if !j.dirty {
		return nil
	}
	log.Printf("FLUSHING Journal (%s)", j.fname)
	fname := j.fname + ".new"
	err := j.writeNew(fname)
	if err != nil {
		return err
	}
	j.dirty = false
	return os.Rename(fname, j.fname)
}

func (j *Journal[T]) tryShutdown() bool {
	j.l.Lock()
	defer j.l.Unlock()
	if !j.isOpen() {
		return false
	}
	close(j.shutdown)
	return true
}

func (j *Journal[T]) Close() error {
	sd := j.tryShutdown()
	if !sd {
		return nil
	}
	log.Printf("Shutting down Journal.")
	j.wg.Wait()
	return j.Flush()
	// l.Lock()
	// 	defer l.Unlock()
	// 	err := j.lockedFlush()
	// 	if err != nil {
	// 		return err
	// 	}
}

func (j *Journal[T]) worker() {
	j.wg.Add(1)
	defer j.wg.Done()
	t := time.NewTicker(j.period)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			j.Flush()
		case <-j.shutdown:
			return
		}
	}
}

func (j *Journal[T]) cindex(i uint64) int {
	return int(i % uint64(len(j.cbuf)))
}

func (j *Journal[T]) Add(v T) (uint64, error) {
	j.l.Lock()
	defer j.l.Unlock()
	if !j.isOpen() {
		return 0, ErrClosed
	}
	ci := j.cindex(j.index) //int(j.index % uint64(len(j.cbuf)))
	ri := j.index
	j.index++
	j.cbuf[ci] = entry[T]{ri, v}
	j.dirty = true
	return ri, nil
}

func (j *Journal[T]) Get(i uint64) (T, error) {
	j.l.RLock()
	defer j.l.RUnlock()
	e := j.cbuf[j.cindex(i)]
	if e.I != i {
		var ret T
		return ret, ErrNotPresent
	}
	return e.E, nil
}

func (j *Journal[T]) Latest() uint64 {
	j.l.RLock()
	defer j.l.RUnlock()
	if j.index > 0 {
		return j.index - 1
	}
	return j.index
}
