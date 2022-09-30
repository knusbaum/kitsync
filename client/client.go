package client

import (
	"context"
	"io"
	"os"

	"github.com/knusbaum/kitsync/ksrpc"
)

func Upload(ctx context.Context, c ksrpc.ControllerClient, fname string, tags map[string]string) (string, error) {
	f, err := os.Open(fname)
	if err != nil {
		//fmt.Printf("Failed to open %s: %v\n", *file, err)
		return "", err
	}
	defer f.Close()

	cli, err := c.Put(ctx)
	if err != nil {
		return "", err
	}

	err = cli.Send(&ksrpc.ContentChunk{Tags: tags})
	if err != nil {
		// 		fmt.Printf("FILE: %s\n", fname)
		// 		fmt.Printf("While sending tags: Failed to send chunk: %v\n", err)
		// 		for k, v := range tags {
		// 			fmt.Printf("[%s]: [%s]\n", k, v)
		// 		}
		return "", err
	}
	for {
		var bbs [4096]byte
		bs := bbs[:]
		n, err := f.Read(bs)
		if err != nil && err != io.EOF {
			//fmt.Printf("Failed to read %s: %v\n", *file, err)
			// TODO: handle closing cli properly
			return "", err
		}
		bs = bs[:n]

		err = cli.Send(&ksrpc.ContentChunk{Data: bs})
		if err != nil {
			//fmt.Printf("FILE: %s\n", *file)
			//fmt.Printf("While sending data (%d bytes): Failed to send chunk: %v\n", len(bs), err)
			return "", err
		}
		if len(bs) == 0 {
			break
		}
	}
	reply, err := cli.CloseAndRecv()
	if err != nil {
		//fmt.Printf("Failed to finish sending: %v\n", err)
		return "", err
	}
	//fmt.Printf("%s\n", reply.ID)
	return reply.ID, nil
}
