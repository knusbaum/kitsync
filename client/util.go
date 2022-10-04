package client

import (
	"fmt"
	"sort"
)

func PrintSortedTags(tags map[string]string) {
	var ks []string
	for k := range tags {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("\t%s: %s\n", k, tags[k])
	}
}
