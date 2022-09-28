package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
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
	tags := make(map[string]string)
	var bs bytes.Buffer
	for {
		cc, err := cas.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if cc.Tags != nil {
			for k, v := range cc.Tags {
				tags[k] = v
			}
		}
		if len(cc.Data) > 0 {
			bs.Write(cc.Data)
		}
	}
	fmt.Printf("TAGS: %#v\n", tags)
	o := kitsync.NewMemObject(bs.Bytes(), tags)
	err := s.c.Put(o)
	if err != nil {
		return err
	}
	return cas.SendAndClose(&ksrpc.AddReply{
		ID: o.ID(),
	})
}

func (s *server) Search(q *ksrpc.Query, cli ksrpc.Controller_SearchServer) error {
	i, err := s.c.Index().Tag(q.Key, q.Value)
	if err != nil {
		return err
	}
	for id, err := i.Next(); err == nil; id, err = i.Next() {
		err := cli.Send(&ksrpc.ID{ID: id})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) Lookup(_ context.Context, id *ksrpc.ID) (*ksrpc.LookupResult, error) {
	o, ok := s.c.Get(id.ID)
	if !ok {
		return &ksrpc.LookupResult{}, nil
	}
	tags, err := o.Tags()
	if err != nil {
		return nil, err
	}
	return &ksrpc.LookupResult{
		Tags:    tags,
		Present: true,
	}, nil
}

func (s *server) AddTags(_ context.Context, r *ksrpc.TagsRequest) (*ksrpc.Void, error) {
	log.Printf("Getting %s", r.ID)
	o, ok := s.c.Get(r.ID)
	if !ok {
		return nil, fmt.Errorf("ID %s does not exist.", r.ID)
	}
	for k, v := range r.Tags {
		log.Printf("Adding Tag %s: %s", k, v)
		err := o.AddTag(k, v)
		if err != nil {
			return nil, err
		}
	}
	log.Printf("Returning success.")
	return &ksrpc.Void{}, nil
}

func (s *server) DelTags(_ context.Context, r *ksrpc.TagsRequest) (*ksrpc.Void, error) {
	log.Printf("Getting %s", r.ID)
	o, ok := s.c.Get(r.ID)
	if !ok {
		return nil, fmt.Errorf("ID %s does not exist.", r.ID)
	}
	for k, v := range r.Tags {
		log.Printf("Deleting Tag %s: %s", k, v)
		err := o.DelTag(k)
		if err != nil {
			return nil, err
		}
	}
	log.Printf("Returning success.")
	return &ksrpc.Void{}, nil
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
	st1, err := kitsync.NewFSStorage("/tmp/foo")
	if err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}
	ct, err := kitsync.NewController("testdata", st1)
	if err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}
	defer ct.Close()
	//ct.AddSecondary(kitsync.NewFSStorage("/mnt/microsoft/testkitsync"))
	st2, err := kitsync.NewFSStorage("/tmp/bar")
	if err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}
	ct.AddSecondary(st2)

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
