package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"

	"net/http"
	_ "net/http/pprof"

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

func (s *server) Put(cas ksrpc.Controller_PutServer) error {
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
	//fmt.Printf("TAGS: %#v\n", tags)
	o := kitsync.NewMemObject(bs.Bytes(), tags)
	err := s.c.Put(o)
	if err != nil {
		return err
	}
	//fmt.Printf("ID: %s\n", o.ID())
	return cas.SendAndClose(&ksrpc.AddReply{
		ID: o.ID(),
	})
}

func (s *server) Search(q *ksrpc.Query, cli ksrpc.Controller_SearchServer) error {
	i, err := s.c.Index().SearchTag(q.Key, q.Value)
	if err != nil {
		log.Printf("Failed to search: %v\n", err)
		return err
	}
	var id string
	for id, err = i.Next(); err == nil; id, err = i.Next() {
		err := cli.Send(&ksrpc.ID{ID: id})
		if err != nil {
			log.Printf("Failed to search: %v\n", err)
			return err
		}
	}
	fmt.Printf("SEARCH: %v\n", err)
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

func (s *server) AddTags(_ context.Context, r *ksrpc.ObjectRequest) (*ksrpc.Void, error) {
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

func (s *server) DelTags(_ context.Context, r *ksrpc.ObjectRequest) (*ksrpc.Void, error) {
	log.Printf("Getting %s", r.ID)
	o, ok := s.c.Get(r.ID)
	if !ok {
		return nil, fmt.Errorf("ID %s does not exist.", r.ID)
	}
	for k, v := range r.Tags {
		if k == "" {
			log.Printf("Deleting Keyless Tag :%s", v)
			err := o.DelTag(":" + v)
			if err != nil {
				return nil, err
			}
		} else {
			log.Printf("Deleting Tag %s: %s", k, v)
			err := o.DelTag(k)
			if err != nil {
				return nil, err
			}
		}
	}
	log.Printf("Returning success.")
	return &ksrpc.Void{}, nil
}

func (s *server) Content(r *ksrpc.ObjectRequest, cli ksrpc.Controller_ContentServer) error {
	log.Printf("Getting %s", r.ID)
	o, ok := s.c.Get(r.ID)
	if !ok {
		return fmt.Errorf("ID %s does not exist.", r.ID)
	}
	c, err := o.Content()
	if err != nil {
		return err
	}
	defer c.Close()
	buf := make([]byte, 8192)
	rd := bufio.NewReader(c)
	for {
		n, err := rd.Read(buf)
		if err != nil {
			if err == io.EOF {
				if n > 0 {
					err := cli.Send(&ksrpc.ContentChunk{Data: buf[:n]})
					if err != nil {
						return err
					}
				}
				break
			}
			return err
		}
		if n > 0 {
			err := cli.Send(&ksrpc.ContentChunk{Data: buf[:n]})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var (
	port = flag.Int("port", 50051, "The server port")
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

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
	// 	st3, err := kitsync.NewFSStorage("/mnt/microsoft/kstest")
	// 	if err != nil {
	// 		log.Fatalf("Failed to start controller: %v", err)
	// 	}
	// 	ct.AddSecondary(st3)

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
