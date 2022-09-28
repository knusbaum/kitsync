package main

import (
	"fmt"
	"os"

	"github.com/blevesearch/bleve/v2"
)

func main() {
	os.RemoveAll("example.bleve")
	// open a new index
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New("example.bleve", mapping)
	if err != nil {
		fmt.Println(err)
		return
	}

	data := struct {
		Name map[string]string
	}{
		Name: map[string]string{
			"foo": "bar",
			"baz": "text",
		},
	}

	// index some data
	index.Index("id", data)

	data.Name = map[string]string{
		"foo": "text",
		"baz": "otre",
	}
	index.Index("id2", data)

	data.Name = map[string]string{
		"foo": "otherThing",
	}
	index.Index("id3", data)

	// search for some text
	query := bleve.NewMatchQuery("")
	query.SetField("Name.baz")
	search := bleve.NewSearchRequest(query)
	searchResults, err := index.Search(search)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(searchResults)
	for i := range searchResults.Hits {
		fmt.Printf("%d: %#v\n", i, searchResults.Hits[i])
	}
}

//
// func test2() {
//
// 	ct, err := kitsync.NewController("testdata", kitsync.NewFSStorage("/tmp/foo"))
// 	if err != nil {
// 		log.Fatalf("Failed to start controller: %v", err)
// 	}
// 	defer ct.Close()
// 	ct.AddSecondary(kitsync.NewFSStorage("/tmp/bar"))
// 	ct.AddSecondary(kitsync.NewFSStorage("/tmp/baz"))
//
// 	// 	err = ct.Put(kitsync.NewMemObject([]byte("Hello, World!")))
// 	// 	if err != nil {
// 	// 		log.Fatal(err)
// 	// 	}
// 	// 	err = ct.Put(kitsync.NewMemObject([]byte("Goodbye, World!")))
// 	// 	if err != nil {
// 	// 		log.Fatal(err)
// 	// 	}
// 	// 	err = ct.Put(kitsync.NewMemObject([]byte("STORAGE1")))
// 	// 	if err != nil {
// 	// 		log.Fatal(err)
// 	// 	}
// 	c := make(chan os.Signal, 1)
// 	signal.Notify(c, os.Interrupt)
// 	<-c
// 	//time.Sleep(30 * time.Second)
// }
//
// func test_1() {
// 	s := kitsync.NewFSStorage("/tmp/foo")
// 	err := s.Put(kitsync.NewMemObject([]byte("Hello, World!")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = s.Put(kitsync.NewMemObject([]byte("Goodbye, World!")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = s.Put(kitsync.NewMemObject([]byte("STORAGE1")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
//
// 	s2 := kitsync.NewFSStorage("/tmp/bar")
// 	err = s2.Put(kitsync.NewMemObject([]byte("Hello, World!")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = s2.Put(kitsync.NewMemObject([]byte("Goodbye, World!")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = s2.Put(kitsync.NewMemObject([]byte("STORAGE2")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
//
// 	s3 := kitsync.NewFSStorage("/tmp/baz")
// 	err = s3.Put(kitsync.NewMemObject([]byte("Hello, World!")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = s3.Put(kitsync.NewMemObject([]byte("Goodbye, World!")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = s3.Put(kitsync.NewMemObject([]byte("STORAGE3")))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
//
// 	fmt.Printf("Storage1:\n")
// 	i, err := s.Index().Iter()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer i.Close()
// 	for n, err := i.Next(); err == nil; n, err = i.Next() {
// 		fmt.Printf("\t%s\n", n)
// 		o, ok := s.Get(n)
// 		if !ok {
// 			log.Fatalf("%s not present", n)
// 		}
// 		c, err := o.Content()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer c.Close()
// 		content, err := io.ReadAll(c)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("\t\t[%s]\n", content)
// 	}
//
// 	fmt.Printf("Storage2:\n")
// 	i, err = s2.Index().Iter()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer i.Close()
// 	for n, err := i.Next(); err == nil; n, err = i.Next() {
// 		fmt.Printf("\t%s\n", n)
// 		o, ok := s2.Get(n)
// 		if !ok {
// 			log.Fatalf("%s not present", n)
// 		}
// 		c, err := o.Content()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer c.Close()
// 		content, err := io.ReadAll(c)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("\t\t[%s]\n", content)
// 	}
//
// 	fmt.Printf("Storage3:\n")
// 	i, err = s3.Index().Iter()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer i.Close()
// 	for n, err := i.Next(); err == nil; n, err = i.Next() {
// 		fmt.Printf("\t%s\n", n)
// 		o, ok := s3.Get(n)
// 		if !ok {
// 			log.Fatalf("%s not present", n)
// 		}
// 		c, err := o.Content()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer c.Close()
// 		content, err := io.ReadAll(c)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("\t\t[%s]\n", content)
// 	}
//
// 	fmt.Printf("Syncing.\n")
// 	ct, err := kitsync.NewController("testdata", s)
// 	if err != nil {
// 		log.Fatalf("Failed to start controller: %v", err)
// 	}
// 	defer ct.Close()
// 	ct.AddSecondary(s2)
// 	ct.AddSecondary(s3)
// 	err = ct.Sync()
// 	if err != nil {
// 		log.Fatalf("Failed to sync: %s", err)
// 	}
//
// 	fmt.Printf("Storage1:\n")
// 	i, err = s.Index().Iter()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer i.Close()
// 	for n, err := i.Next(); err == nil; n, err = i.Next() {
// 		fmt.Printf("\t%s\n", n)
// 		o, ok := s.Get(n)
// 		if !ok {
// 			log.Fatalf("%s not present", n)
// 		}
// 		c, err := o.Content()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer c.Close()
// 		content, err := io.ReadAll(c)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("\t\t[%s]\n", content)
// 	}
//
// 	fmt.Printf("Storage2:\n")
// 	i, err = s2.Index().Iter()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer i.Close()
// 	for n, err := i.Next(); err == nil; n, err = i.Next() {
// 		fmt.Printf("\t%s\n", n)
// 		o, ok := s2.Get(n)
// 		if !ok {
// 			log.Fatalf("%s not present", n)
// 		}
// 		c, err := o.Content()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer c.Close()
// 		content, err := io.ReadAll(c)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("\t\t[%s]\n", content)
// 	}
//
// 	fmt.Printf("Storage3:\n")
// 	i, err = s3.Index().Iter()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer i.Close()
// 	for n, err := i.Next(); err == nil; n, err = i.Next() {
// 		fmt.Printf("\t%s\n", n)
// 		o, ok := s3.Get(n)
// 		if !ok {
// 			log.Fatalf("%s not present", n)
// 		}
// 		c, err := o.Content()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer c.Close()
// 		content, err := io.ReadAll(c)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("\t\t[%s]\n", content)
// 	}
//
// }
