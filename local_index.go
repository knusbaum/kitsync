package kitsync

import (
	"fmt"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

type fsIndex struct {
	root   string
	tipath string
	tindex bleve.Index
	l      sync.RWMutex
}

func openFSIndex(root string) (*fsIndex, error) {
	idx := &fsIndex{
		root:   root,
		tipath: path.Join(root, "tag_index.bleve"),
	}
	_, err := os.Stat(idx.tipath)
	if os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
		index, err := bleve.New(idx.tipath, mapping)
		if err != nil {
			return nil, err
		}
		idx.tindex = index
	} else {
		index, err := bleve.Open(idx.tipath)
		if err != nil {
			return nil, err
		}
		idx.tindex = index
	}
	return idx, nil
}

func (i *fsIndex) Close() error {
	return i.tindex.Close()
}

func (i *fsIndex) Iter() (Iterator, error) {
	return newFSIndexIterator(i.root)
}

func (i *fsIndex) Present(hash string) bool {
	if !validHash(hash) {
		return false
	}
	_, err := os.Stat(path.Join(i.root, pathForHash(hash)))
	if err != nil {
		return false
	}
	return true
}

func (i *fsIndex) SearchTag(k, v string) (Iterator, error) {
	var q query.Query
	if v == "" {
		query := bleve.NewPrefixQuery("")
		if k != "" {
			query.SetField("Tags." + k)
		}
		q = query
	} else {
		query := bleve.NewMatchQuery(v)
		if k != "" {
			query.SetField("Tags." + k)
		}
		q = query
	}
	fmt.Printf("SEARCHING: %#v\n", q)
	search := bleve.NewSearchRequest(q)
	search.Size = math.MaxInt
	searchResults, err := i.tindex.Search(search)
	if err != nil {
		return nil, err
	}
	sort.Sort((*ByDocumentID)(searchResults))
	return &bleveResultsIterator{r: searchResults}, nil
}

func (i *fsIndex) Add(o Object) error {
	tags, err := o.Tags()
	if err != nil {
		return err
	}
	data := struct {
		Tags map[string]string
	}{
		Tags: tags,
	}
	//fmt.Printf("Indexing... %#v\n", data)
	i.tindex.Index(o.ID(), data)
	return nil
}

type bleveResultsIterator struct {
	r *bleve.SearchResult
	i int
}

func (i *bleveResultsIterator) Next() (string, error) {
	if i.i >= len(i.r.Hits) {
		return "", itDone
	}
	id := i.r.Hits[i.i].ID
	i.i++
	return id, nil
}
func (i *bleveResultsIterator) Close() error {
	return nil
}

type ByDocumentID bleve.SearchResult

func (d *ByDocumentID) Len() int { return len(d.Hits) }

func (d *ByDocumentID) Less(i, j int) bool {
	return strings.Compare(d.Hits[i].ID, d.Hits[j].ID) < 0
}

func (d *ByDocumentID) Swap(i, j int) {
	d.Hits[i], d.Hits[j] = d.Hits[j], d.Hits[i]
}

type ByNameAlpha []os.DirEntry

func (b ByNameAlpha) Len() int { return len(b) }

func (b ByNameAlpha) Less(i, j int) bool {
	return strings.Compare(b[i].Name(), b[j].Name()) < 0
}

func (b ByNameAlpha) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type fsIndexIterator struct {
	// 	i       *fsIndex
	root    string
	members []os.DirEntry
	subIt   *fsIndexIterator
}

func newFSIndexIterator(p string) (*fsIndexIterator, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	des, err := f.ReadDir(0)
	if err != nil {
		return nil, err
	}
	sort.Sort(ByNameAlpha(des))
	return &fsIndexIterator{root: p, members: des}, nil
	//i.members = des
}

var itDone = fmt.Errorf("Finished")

func (i *fsIndexIterator) Next() (string, error) {
	if i.subIt != nil {
		h, err := i.subIt.Next()
		if err == nil {
			return h, err
		} else if err == itDone {
			err = i.subIt.Close()
			if err != nil {
				return "", err
			}
			i.subIt = nil
		} else {
			return "", err
		}
	}
	if len(i.members) == 0 {
		return "", itDone
	}
	m := i.members[0]
	i.members = i.members[1:]
	if m.IsDir() {
		si, err := newFSIndexIterator(path.Join(i.root, m.Name()))
		if err != nil {
			return "", err
		}
		i.subIt = si
		return i.Next()
	} else if path.Ext(m.Name()) != "" { //else if strings.HasSuffix(m.Name(), "tags") {
		// skip
		return i.Next()
	} else {
		return m.Name(), nil
	}
}

func (i *fsIndexIterator) Close() error {
	i.subIt = nil
	i.members = nil
	return nil
}

//
// 	os.RemoveAll("example.bleve")
// 	// open a new index
// 	mapping := bleve.NewIndexMapping()
// 	index, err := bleve.New("example.bleve", mapping)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
//
// 	data := struct {
// 		Name map[string]string
// 	}{
// 		Name: map[string]string{
// 			"text": "bar",
// 			"baz":  "boo",
// 		},
// 	}
//
// 	// index some data
// 	index.Index("id", data)
//
// 	// search for some text
// 	query := bleve.NewMatchQuery("text")
// 	search := bleve.NewSearchRequest(query)
// 	searchResults, err := index.Search(search)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	fmt.Println(searchResults)
