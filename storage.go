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
	// AddTag adds a "tag" - a key/value pair to the Object. In general, Keys and tags must
	// not contain colons, spaces, nor commas. (':', ' ', ',') See key-less tags below for an
	// exception to this rule. Otherwise, the range of UTF-8 is available.
	//
	// AddTag may be called with an empty key, in which case, the tag will be added to the
	// special key-less tag list. This key-less tag list will show up in Tags() as a
	// space-separated string of all the key-less tags added to the object. Key-less tags that
	// contain spaces will me treated as multiple key-less tags separated by spaces.
	AddTag(string, string) error
	// DelTag deletes a tag by key name.
	//
	// If the argument to DelTag is preceeded by a colon (':'), then the argument will be interpreted
	// as a key-less tag to be removed from the object's list of key-less tags. As with AddTag,
	// key-less tags containing spaces will be interpreted as multiple key-less tags
	DelTag(string) error
	Content() (io.ReadCloser, error)
}

type Iterator interface {
	Next() (string, error)
	Close() error
}

type IndexedStorage interface {
	Storage
	Syncer
	// Reindex causes the index to be dropped and recreated from scratch
	Reindex() error
	IDs() (Iterator, error)
	Indexed(id string) bool
	SearchTag(k, v string) (Iterator, error)
	Keys() (Iterator, error)
	Tags() (Iterator, error)
}

type Storage interface {
	Put(Object) error
	Get(id string) (Object, bool)
	Delete(id string) error
	Present(id string) bool
	Iter() (Iterator, error)
	Close() error
	fmt.Stringer
}

type ControllableStorage interface {
	Storage
	Checkpoint() uint64
	SetCheckpoint(c uint64)
	Cleanup()
}

type Syncer interface {
	// Sync may take an extremely long time
	Sync() error
}
