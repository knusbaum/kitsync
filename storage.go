package kitsync

import (
	"fmt"
	"io"
	"time"
)

type Object interface {
	// ID must return a hex-encoded (encoding/hex.EncodeToString) sha256 hash
	// of the object content.
	ID() string
	// Hash must return a hex-encoded (encoding/hex.EncodeToString) sha256 hash
	// of the ID and the Tags.
	//
	// To add a tag to the hash, add the key, then the value (h.Write(key); h.Write(value);)
	// Keys and values must be added in sorted order to have a consistent hash, according to sort.Strings.
	Hash() (string, error)
	// Updated is the date the object was last updated. Used to resolve conflicts
	Updated() (time.Time, error)
	Tags() (map[string]string, error)
	AddTag(string, string) error
	Content() (io.ReadCloser, error)
}

type IndexIterator interface {
	Next() (string, error)
	Close() error
}

type Index interface {
	Iter() (IndexIterator, error)
	Present(hash string) bool
	Tag(k, v string) (IndexIterator, error)
}

type Storage interface {
	Put(Object) error
	Get(hash string) (Object, bool)
	Index() Index
	Checkpoint() uint64
	SetCheckpoint(c uint64)
	fmt.Stringer
}
