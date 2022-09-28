package kitsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"
)

type memObject struct {
	content []byte
	tags    map[string]string
	updated time.Time
}

func (o *memObject) ID() string {
	h := sha256.New()
	h.Write(o.content)
	s := h.Sum(nil)
	return hex.EncodeToString(s)
}

func (o *memObject) Hash() (string, error) {
	s := o.ID()
	return calculateHash(s, o.tags), nil
}

func (o *memObject) Updated() (time.Time, error) {
	return o.updated, nil
}

func (o *memObject) Tags() (map[string]string, error) {
	m := make(map[string]string, len(o.tags))
	for k, v := range o.tags {
		m[k] = v
	}
	return m, nil
}

func (o *memObject) AddTag(k, v string) error {
	o.tags[k] = v
	return nil
}

func (o *memObject) Content() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBuffer(o.content)), nil
}

func NewMemObject(bs []byte) Object {
	return &memObject{content: bs, updated: time.Now()}
}
