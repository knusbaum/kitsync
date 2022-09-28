package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/knusbaum/kitsync"
	"github.com/knusbaum/kitsync/ksrpc"
	"google.golang.org/grpc"
)

type server struct {
	c *kitsync.Controller
	ksrpc.UnimplementedControllerServer
}

func (s *server) Sync(context.Context, *ksrpc.Void) (*ksrpc.Void, error) {
	err := s.c.Sync()
	if err != nil {
		return nil, err
	}
	return &ksrpc.Void{}, nil
}

func (s *server) Add(cas ksrpc.Controller_AddServer) error {
	var bs bytes.Buffer
	for {
		cc, err := cas.Recv()
		if err != nil {
			return err
		}
		if len(cc.Data) > 0 {
			bs.Write(cc.Data)
		} else {
			break
		}
	}
	o := kitsync.NewMemObject(bs.Bytes())
	err := s.c.Put(o)
	if err != nil {
		return err
	}
	return cas.SendAndClose(&ksrpc.AddReply{
		ID: o.ID(),
	})
}

var (
	port = flag.Int("port", 50051, "The server port")
)

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	ct, err := kitsync.NewController("testdata", kitsync.NewFSStorage("/tmp/foo"))
	if err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}
	defer ct.Close()
	//ct.AddSecondary(kitsync.NewFSStorage("/mnt/microsoft/testkitsync"))
	ct.AddSecondary(kitsync.NewFSStorage("/tmp/bar"))

	s := grpc.NewServer()
	ksrpc.RegisterControllerServer(s, &server{c: ct})
	log.Printf("server listening at %v", lis.Addr())
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	//time.Sleep(30 * time.Second)

	// 	h := &ksrpc.Handler{}
	// 	rpc.Register(h)
	// 	rpc.HandleHTTP()
	// 	l, e := net.Listen("tcp", ":61234")
	// 	if e != nil {
	// 		log.Fatal("listen error:", e)
	// 	}
	// 	go http.Serve(l, nil)
	// 	for {
	// 		time.Sleep(5 * time.Second)
	// 		fmt.Printf("%#v\n", h)
	// 	}
}
