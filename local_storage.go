package kitsync

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

type fsObject struct {
	path    string
	id      string
	hash    string
	updated time.Time
	tags    map[string]string
	l       sync.RWMutex
	s       *fsStorage
}

func (o *fsObject) ID() string {
	return o.id
}

func (o *fsObject) Hash() (string, error) {
	if o.hash == "" {
		if err := o.readHash(); err != nil {
			return "", err
		}
	}
	return o.hash, nil
}

func (o *fsObject) readHash() error {
	f, err := os.Open(o.path + ".hash")
	if err != nil {
		return err
	}
	defer f.Close()
	bs, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	o.hash = string(bs)
	return nil
}

func calculateHash(id string, tags map[string]string) string {
	var ks []string
	for k := range tags {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	h := sha256.New()
	_, err := io.WriteString(h, id)
	if err != nil {
		panic(err)
	}
	for _, k := range ks {
		_, err = io.WriteString(h, k)
		if err != nil {
			panic(err)
		}
	}
	s := h.Sum(nil)
	return hex.EncodeToString(s)
}

func (o *fsObject) writeHash() error {
	id := o.ID()
	f, err := os.Open(o.path + ".tags")
	if err != nil {
		return err
	}
	defer f.Close()

	err = o.loadTags()
	if err != nil {
		return err
	}
	o.hash = calculateHash(id, o.tags)

	f, err = os.Create(o.path + ".hash")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, o.hash)
	return err
}

func (o *fsObject) Updated() (time.Time, error) {
	if o.updated.IsZero() {
		f, err := os.Open(o.path + ".updated")
		if err != nil {
			return time.Time{}, err
		}
		defer f.Close()

		bs, err := io.ReadAll(f)
		if err != nil {
			return time.Time{}, err
		}
		t, err := time.Parse(time.RFC3339, string(bs))
		if err != nil {
			return time.Time{}, err
		}
		o.updated = t
	}
	return o.updated, nil
}

// Tags returns a copy of the object's tags. The copy is safe to modify, but modification have no
// effect on the object's tags.
func (o *fsObject) Tags() (map[string]string, error) {
	err := o.loadTags()
	if err != nil {
		return nil, err
	}
	o.l.RLock()
	defer o.l.RUnlock()
	m := make(map[string]string, len(o.tags))
	for k, v := range o.tags {
		m[k] = v
	}
	return m, nil
}

func (o *fsObject) nilTags() bool {
	o.l.RLock()
	defer o.l.RUnlock()
	return o.tags == nil
}

func (o *fsObject) loadTags() error {
	if !o.nilTags() {
		return nil
	}
	o.l.Lock()
	defer o.l.Unlock()
	return o.lockedLoadTags()
}

func (o *fsObject) lockedLoadTags() error {
	// We have to check again, since we didn't have the lock before (something else could have loaded tags)
	if o.tags != nil {
		return nil
	}
	f, err := os.Open(o.path + ".tags")
	if err != nil {
		return err
	}
	defer f.Close()

	m := make(map[string]string)
	d := gob.NewDecoder(f)
	err = d.Decode(&m)
	if err != nil {
		return err
	}
	o.tags = m
	return nil
}

func addKeyless(existing, new string) string {
	es := strings.Split(existing, " ")
	news := strings.Split(new, " ")
out:
	for _, n := range news {
		n = strings.TrimSpace(n)
		for _, e := range es {
			if e == n {
				continue out
			}
		}
		es = append(es, n)
	}
	fmt.Printf("Added %#v to %s: %#v\n", news, existing, es)
	return strings.TrimSpace(strings.Join(es, " "))
}

func removeKeyless(existing, remove string) string {
	es := strings.Split(existing, " ")
	rems := strings.Split(remove, " ")
	for _, n := range rems {
		n = strings.TrimSpace(n)
		k := 0
		for ei, e := range es {
			if e == n {
				continue
			}
			es[k] = es[ei]
			k++
		}
		es = es[:k]
	}
	fmt.Printf("Removed %#v from %s: %#v\n", rems, existing, es)
	return strings.Join(es, " ")
}

func (o *fsObject) addTag(k, v string) error {
	o.l.Lock()
	defer o.l.Unlock()
	err := o.lockedLoadTags()
	if err != nil {
		return err
	}
	if k == "" {
		o.tags[""] = addKeyless(o.tags[""], v)
	} else {
		o.tags[k] = v
	}
	f, err := os.Create(o.path + ".tags")
	if err != nil {
		return err
	}
	defer f.Close()

	e := gob.NewEncoder(f)
	err = e.Encode(o.tags)
	if err != nil {
		return err
	}
	return nil
}

func (o *fsObject) AddTag(k, v string) error {
	err := o.addTag(k, v)
	if err != nil {
		return err
	}

	// 	// Update the index
	// 	err = o.s.idx.Add(o)
	// 	if err != nil {
	// 		log.Printf("(AddTag) Failed to index tags for %s: %v", o.id, err)
	// 	}

	return nil
}

func (o *fsObject) delTag(k string) error {
	o.l.Lock()
	defer o.l.Unlock()
	err := o.lockedLoadTags()
	if err != nil {
		return err
	}
	if strings.HasPrefix(k, ":") {
		k = strings.TrimPrefix(k, ":")
		if v := removeKeyless(o.tags[""], k); v != "" {
			o.tags[""] = v
		} else {
			delete(o.tags, "")
		}
	} else {
		delete(o.tags, k)
	}
	f, err := os.Create(o.path + ".tags")
	if err != nil {
		return err
	}
	defer f.Close()

	e := gob.NewEncoder(f)
	err = e.Encode(o.tags)
	if err != nil {
		return err
	}
	return nil
}

func (o *fsObject) DelTag(k string) error {
	err := o.delTag(k)
	if err != nil {
		return err
	}

	// 	// Update the index
	// 	err = o.s.idx.Add(o)
	// 	if err != nil {
	// 		log.Printf("(DelTag) Failed to index tags for %s: %v", o.id, err)
	// 	}

	return nil
}

func (o *fsObject) Content() (io.ReadCloser, error) {
	f, err := os.Open(o.path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func validHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return false
	}
	return true
}

func pathForHash(hash string) string {
	p1 := hash[:4]
	p2 := hash[4:8]
	p3 := hash[8:12]
	return path.Join(p1, p2, p3, hash)
}

type fsStorage struct {
	root string
	//idx  *fsIndex
	//idx *psqlIndex
}

func (s *fsStorage) String() string {
	return fmt.Sprintf("Local(%s)", s.root)
}

func (s *fsStorage) Put(o Object) error {
	h := o.ID()
	if s.Present(h) {
		// Already present
		return nil
	}
	op := path.Join(s.root, pathForHash(h))
	err := os.MkdirAll(path.Dir(op), 0770)
	if err != nil {
		return err
	}

	// Write content
	f, err := os.Create(op)
	if err != nil {
		return err
	}
	defer f.Close()

	cr, err := o.Content()
	if err != nil {
		return err
	}
	defer cr.Close()

	_, err = io.Copy(f, cr)
	if err != nil {
		os.Remove(op)
		return err
	}

	// 	// Update the index
	// 	err = s.idx.Add(o)
	// 	if err != nil {
	// 		log.Printf("(Put) Failed to index tags for %s: %v", h, err)
	// 	}

	// Write updated file
	f, err = os.Create(op + ".updated")
	if err != nil {
		os.Remove(op)
		return err
	}
	defer f.Close()
	t, err := o.Updated()
	if err != nil {
		os.Remove(op)
		os.Remove(op + ".updated")
		return err
	}
	_, err = io.WriteString(f, t.Format(time.RFC3339))
	if err != nil {
		os.Remove(op)
		os.Remove(op + ".updated")
		return err
	}

	// Write tags file
	f, err = os.Create(op + ".tags")
	if err != nil {
		return err
	}
	defer f.Close()

	tags, err := o.Tags()
	if err != nil {
		return err
	}

	e := gob.NewEncoder(f)
	err = e.Encode(tags)
	if err != nil {
		return err
	}
	return nil
}

func (s *fsStorage) Delete(id string) error {
	ofile := path.Join(s.root, pathForHash(id))
	if err := os.Remove(ofile); err != nil {
		return err
	}
	os.Remove(ofile + ".tags")
	os.Remove(ofile + ".updated")
	os.Remove(ofile + ".hash")
	return nil
}

func readD(d string, n int) ([]os.DirEntry, error) {
	f, err := os.Open(d)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	es, err := f.ReadDir(n)
	if err != nil {
		return nil, err
	}
	return es, nil
}

func (s *fsStorage) cleanup(d string) {
	es, err := readD(d, 0)
	if err != nil {
		if err == io.EOF {
			// dir is empty.
			fmt.Printf("Removing %s\n", d)
			os.Remove(d)
		}
		return
	}

	for _, e := range es {
		if e.IsDir() {
			s.cleanup(path.Join(d, e.Name()))
		}
	}

	es, err = readD(d, 1)
	if err != nil {
		if err == io.EOF {
			// dir is empty.
			fmt.Printf("Removing %s\n", d)
			os.Remove(d)
		}
		return
	}
}

func (s *fsStorage) Cleanup() {
	s.cleanup(s.root)
}

func (s *fsStorage) Present(id string) bool {
	p := path.Join(s.root, pathForHash(id))
	if _, err := os.Stat(p); err != nil {
		return false
	}
	return true
}

func (s *fsStorage) Get(id string) (Object, bool) {
	if s.Present(id) {
		o := &fsObject{
			id:   id,
			path: path.Join(s.root, pathForHash(id)),
			s:    s,
		}
		return o, true
	}
	return nil, false
}

func (s *fsStorage) Iter() (Iterator, error) {
	return newFSIndexIterator(s.root)
}

func (s *fsStorage) Checkpoint() uint64 {
	f, err := os.Open(path.Join(s.root, "checkpoint.id"))
	if err != nil {
		return 0
	}
	defer f.Close()
	d := gob.NewDecoder(f)
	var i uint64
	err = d.Decode(&i)
	if err != nil {
		return 0
	}
	return i
}

func (s *fsStorage) SetCheckpoint(i uint64) {
	f, err := os.Create(path.Join(s.root, "checkpoint.id"))
	if err != nil {
		log.Printf("Failed to set checkpoint on %s: %v", s, err)
		return
	}
	defer f.Close()
	//io.WriteString(f, c)
	//return
	e := gob.NewEncoder(f)
	e.Encode(i)
}

func (s *fsStorage) Close() error {
	//return s.idx.Close()
	return nil
}

func NewFSStorage(root string) (*fsStorage, error) {
	// 	idx, err := openFSIndex(path)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	idx, err := newPsqlIndex("127.0.0.1", 5432, "postgres", "example", strings.Replace(path, "/", "_", -1))
	// 	if err != nil {
	// 		return nil, err
	// 	}

	return &fsStorage{
		root: path.Clean(root),
		//idx:  idx,
	}, nil
}
