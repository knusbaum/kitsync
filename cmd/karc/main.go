package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/knusbaum/kitsync"
	"github.com/knusbaum/kitsync/client"
	"github.com/knusbaum/kitsync/cmd/ks/meta"
	"github.com/knusbaum/kitsync/ksrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr       = flag.String("addr", "localhost:50051", "the address to connect to")
	dir        = flag.String("dir", "", "The directory to archive")
	argtags    = flag.String("tags", "", "Extra tags to be provided on upload")
	discovered = flag.Bool("add-discovered", false, "Automatic yes to adding discovered tags to new files")
	yes        = flag.Bool("y", false, "Automatic yes to uploading new files")
)

func errorf(s string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, s, a...)
}

func failf(s string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, s, a...)
	os.Exit(1)
}

func readDirs(d string) ([]os.DirEntry, error) {
	df, err := os.Open(d)
	if err != nil {
		return nil, err
	}
	defer df.Close()
	return df.ReadDir(-1)
}

func fileHash(fname string) (string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return kitsync.HashReader(f)
}

var in = bufio.NewReader(os.Stdin)

func promptYN(s string, def bool) bool {
	for {
		fmt.Printf("%s (Y/n): ", s)
		l, err := in.ReadString('\n')
		if err != nil {
			errorf("Failed to answer prompt: %s\n", err)
			continue
		}
		l = strings.ToLower(strings.TrimSpace(l))
		switch l {
		case "":
			return def
		case "y":
			fallthrough
		case "yes":
			return true
		case "n":
			fallthrough
		case "no":
			return false
		}
		b, err := strconv.ParseBool(l)
		if err != nil {
			errorf("Don't understand \"%s\"\nTry again...\n", l)
			continue
		}
		return b
	}
}

func promptTags(s string) (map[string]string, error) {
	for {
		fmt.Printf("%s (Comma-separated list of colon-separated key/value pairs): ", s)
		l, err := in.ReadString('\n')
		if err != nil {

			return nil, err
		}
		l = strings.TrimSpace(l)
		ts, err := parseTags(l)
		if err != nil {
			errorf("Failed to parse tags: %s\nTry again...\n", err)
			continue
		}
		return ts, nil
	}
}

// MergeTags adds all the tags in src to dst. Tags already present in dst are overwritten.
func MergeTags(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

func printSortedTags(tags map[string]string) {
	var ks []string
	for k := range tags {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("\t%s: %s\n", k, tags[k])
	}
}

func addFile(c ksrpc.ControllerClient, f string) {
	id, err := fileHash(f)
	if err != nil {
		errorf("Failed to hash %s: %s\n", f, err)
		return
	}

	r, err := c.Lookup(context.Background(), &ksrpc.ID{ID: id})
	if err != nil {
		failf("Failed to lookup %s: %v\n", id, err)
		return
	}
	if r.Present {
		fmt.Printf("%s already exists with tags:\n", id)
		// 		for k, v := range r.Tags {
		// 			fmt.Printf("\t%s: %s\n", k, v)
		// 		}
		printSortedTags(r.Tags)
		return
	}

	if !(*yes || promptYN(fmt.Sprintf("Add file %s (%s)?", f, id), true)) {
		return
	}

	var tags map[string]string
	mtags := meta.GetTags(f)
	// MergeTags(mtags, parseTags(*tags))
	// 	// Make sure all the tags are valid
	vtags := make(map[string]string)
	for k, v := range mtags {
		nk := strings.ToValidUTF8(k, "")
		nv := strings.ToValidUTF8(v, "")
		vtags[nk] = nv
		//fmt.Printf("\t%s: %s\n", nk, nv)
	}
	printSortedTags(vtags)

	if *discovered || promptYN("Add discovered tags?", true) {
		tags = vtags
	}

	var htags map[string]string
	if *argtags == "" {
		htags, err = promptTags("Add additional tags?")
		if err != nil {
			errorf("Failed to answer prompt: %s\nSkipping %s\n", err, f)
			return
		}
	} else {
		htags, err = parseTags(*argtags)
		if err != nil {
			errorf("Failed to parse tags: %s\nTry again...\n", err)
			return
		}
	}

	MergeTags(tags, htags)
	fmt.Printf("\n##########\n%s (%s)\nTags:\n", f, id)
	// 	for k, v := range tags {
	// 		fmt.Printf("\t%s: %s\n", k, v)
	// 	}
	printSortedTags(tags)

	if promptYN("Upload?", true) {
		rid, err := client.Upload(context.Background(), c, f, tags)
		if err != nil {
			errorf("Failed to upload! %s\n", err)
			return
		}
		if id != rid {
			errorf("Expected ids to match, but local was (%s), remote was (%s).\n")
			return
		}
		fmt.Printf("Successfully uploaded %s (%s)\n", f, id)
	}
}

func recursiveArchive(c ksrpc.ControllerClient, d string) {
	var nextDirs []string
	fs, err := readDirs(d)
	if err != nil {
		errorf("Failed to read directory %s: %v\n", d, err)
		return
	}
	for _, fi := range fs {
		if fi.IsDir() {
			nextDirs = append(nextDirs, path.Join(d, fi.Name()))
			continue
		}
		addFile(c, path.Join(d, fi.Name()))
	}
	for _, d := range nextDirs {
		recursiveArchive(c, d)
	}
}

func parseTags(s string) (map[string]string, error) {
	m := make(map[string]string)
	parts := strings.Split(s, ",")
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("Unable to parse tag: %s", p)
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		m[k] = v
	}
	return m, nil
}

func main() {
	flag.Parse()
	if *dir == "" {
		failf("Must specify -dir\n")
	}

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		failf("did not connect: %v", err)
	}
	defer conn.Close()
	c := ksrpc.NewControllerClient(conn)

	recursiveArchive(c, *dir)
}
