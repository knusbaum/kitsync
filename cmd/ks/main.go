package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/knusbaum/kitsync/ksrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr = flag.String("addr", "localhost:50051", "the address to connect to")
	file = flag.String("file", "", "the file to add to ksd")
	s    = flag.Bool("sync", false, "causes ksd to manually sync all storages")
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
		fmt.Printf("Received reply: %#v\n", reply.ID)

	}
}

// func main() {
// 	client, err := rpc.DialHTTP("tcp", "localhost:61234")
// 	if err != nil {
// 		log.Fatal("dialing:", err)
// 	}
//
// 	err = client.Call("Handler.Add", 10, nil)
// 	if err != nil {
// 		log.Fatal("arith error:", err)
// 	}
//
// 	var s ksrpc.Stats
// 	err = client.Call("Handler.Stats", struct{}{}, &s)
// 	if err != nil {
// 		log.Fatal("arith error:", err)
// 	}
// 	fmt.Printf("Stats: %#v\n", s)
// }
