package client

import (
	"fmt"
	"sort"
	"strings"
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

// MergeTags adds all the tags in src to dst. Tags already present in dst are overwritten.
func MergeTags(dst, src map[string]string) {
	for k, v := range src {
		if k == "" {
			dst[k] = MergeKeyless(dst[k], src[k])
		} else {
			dst[k] = v
		}
	}
}

func MergeKeyless(existing, new string) string {
	es := strings.Split(existing, " ")
	news := strings.Split(new, " ")
out:
	for _, n := range news {
		n = strings.TrimSpace(n)
		for _, e := range es {
			if e == n {
				continue out
			}
		}
		es = append(es, n)
	}
	fmt.Printf("Added %#v to %s: %#v\n", news, existing, es)
	return strings.TrimSpace(strings.Join(es, " "))
}

func RemoveKeyless(existing, remove string) string {
	es := strings.Split(existing, " ")
	rems := strings.Split(remove, " ")
	for _, n := range rems {
		n = strings.TrimSpace(n)
		k := 0
		for ei, e := range es {
			if e == n {
				continue
			}
			es[k] = es[ei]
			k++
		}
		es = es[:k]
	}
	fmt.Printf("Removed %#v from %s: %#v\n", rems, existing, es)
	return strings.Join(es, " ")
}
