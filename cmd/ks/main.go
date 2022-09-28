package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/knusbaum/kitsync/ksrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr   = flag.String("addr", "localhost:50051", "the address to connect to")
	file   = flag.String("file", "", "the file to add to ksd")
	s      = flag.Bool("sync", false, "causes ksd to manually sync all storages")
	search = flag.String("search", "", "colon-separated key/value pair to search for.")
	id     = flag.String("id", "", "Without other args, gets the tags for an ID.")
	setkey = flag.String("setkey", "", "colon-separated key/value pair to set on -id.")
	delkey = flag.String("delkey", "", "key to delete on -id.")
)

func main() {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := ksrpc.NewControllerClient(conn)

	if *s {
		_, err = c.Sync(context.Background(), &ksrpc.Void{})
		if err != nil {
			log.Fatalf("failed to sync: %v", err)
		}
	}
	if *file != "" {
		s, err := os.Stat(*file)
		if err != nil {
			fmt.Printf("Failed to open %s: %v\n", *file, err)
			return
		}
		if s.IsDir() {
			fmt.Printf("Cannot upload directories (yet).")
			return
		}
		f, err := os.Open(*file)
		if err != nil {
			fmt.Printf("Failed to open %s: %v\n", *file, err)
			return
		}

		cli, err := c.Add(context.Background())
		if err != nil {
			fmt.Printf("Failed to add %s: %v\n", *file, err)
		}

		tags := map[string]string{
			"filename": path.Base(*file),
		}
		err = cli.Send(&ksrpc.ContentChunk{Tags: tags})
		if err != nil {
			fmt.Printf("Failed to send chunk: %v\n", err)
			return
		}
		for {
			var bbs [4096]byte
			bs := bbs[:]
			n, err := f.Read(bs)
			if err != nil && err != io.EOF {
				fmt.Printf("Failed to read %s: %v\n", *file, err)
				// TODO: handle closing cli properly
				return
			}
			bs = bs[:n]

			err = cli.Send(&ksrpc.ContentChunk{Data: bs})
			if err != nil {
				fmt.Printf("Failed to send chunk: %v\n", err)
				return
			}
			if len(bs) == 0 {
				break
			}
		}
		reply, err := cli.CloseAndRecv()
		if err != nil {
			fmt.Printf("Failed to finish sending: %v\n", err)
			return
		}
		fmt.Printf("%s\n", reply.ID)

	}
	if *search != "" {
		parts := strings.SplitN(*search, ":", 2)
		var key, value string
		if len(parts) == 2 {
			key = parts[0]
			value = parts[1]
		} else {
			value = parts[0]
		}
		cli, err := c.Search(context.Background(), &ksrpc.Query{Key: key, Value: value})
		if err != nil {
			fmt.Printf("Failed to search %s: %v\n", *search, err)
			return
		}

		for id, err := cli.Recv(); err == nil; id, err = cli.Recv() {
			fmt.Printf("%s\n", id.ID)
		}
	}
	if *id != "" {
		if *setkey != "" {
			parts := strings.SplitN(*setkey, ":", 2)
			var key, value string
			if len(parts) != 2 {
				fmt.Printf("Expected a key and value, separated by a colon, but found: \"%s\".\n", *setkey)
				return
			}
			key = parts[0]
			value = parts[1]
			_, err := c.AddTags(context.Background(), &ksrpc.TagsRequest{ID: *id, Tags: map[string]string{key: value}})
			if err != nil {
				fmt.Printf("Failed to set tags on %s: %v\n", *id, err)
				return
			}
		} else if *delkey != "" {
			_, err := c.DelTags(context.Background(), &ksrpc.TagsRequest{ID: *id, Tags: map[string]string{*delkey: ""}})
			if err != nil {
				fmt.Printf("Failed to set tags on %s: %v\n", *id, err)
				return
			}
		}
		//else {
		r, err := c.Lookup(context.Background(), &ksrpc.ID{ID: *id})
		if err != nil {
			fmt.Printf("Failed to get tags %s: %v\n", *search, err)
			return
		}
		if r.Present {
			fmt.Printf("Found %s\n", *id)
			for k, v := range r.Tags {
				fmt.Printf("\t%s: %s\n", k, v)
			}
		} else {
			fmt.Printf("Not Found %s\n", *id)
		}
		//}
	} else if *setkey != "" || *delkey != "" {
		fmt.Printf("Need to specify -id to set/delete tags.\n")
	}

}
