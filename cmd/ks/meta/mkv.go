package meta

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/remko/go-mkvparse"
)

type mkvHandler struct {
	tagName string
	tags    map[string]string
}

var _ mkvparse.Handler = &mkvHandler{}

func getMKVTags(fname string) (map[string]string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tags := make(map[string]string)
	err = mkvparse.ParseSections(f, &mkvHandler{tags: tags}, mkvparse.TagsElement)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

var re *regexp.Regexp = regexp.MustCompile(`([0-9]+):([0-9]+):([0-9]+)\.([0-9]+)`)

func (m *mkvHandler) HandleMasterBegin(id mkvparse.ElementID, e mkvparse.ElementInfo) (bool, error) {
	return true, nil
}
func (m *mkvHandler) HandleMasterEnd(id mkvparse.ElementID, e mkvparse.ElementInfo) error { return nil }
func (m *mkvHandler) HandleString(id mkvparse.ElementID, s string, e mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.TagNameElement:
		m.tagName = s
	case mkvparse.TagStringElement:
		//fmt.Printf("%s: %v\n", m.tagName, s)
		m.tags["mkv."+strings.ToLower(m.tagName)] = s
		if m.tagName == "DURATION" {
			m.tags["duration"] = s
			ms := re.FindStringSubmatch(s)
			fmt.Printf("MATCH: %v\n", ms)
			if ms != nil {
				h, err := strconv.Atoi(ms[1])
				if err != nil {
					fmt.Printf("ERROR: %s: %s\n", ms[1], err)
					return nil
				}
				min, err := strconv.Atoi(ms[2])
				if err != nil {
					fmt.Printf("ERROR: %s: %s\n", ms[2], err)
					return nil
				}
				sec, err := strconv.Atoi(ms[3])
				if err != nil {
					fmt.Printf("ERROR: %s: %s\n", ms[3], err)
					return nil
				}
				nsec, err := strconv.Atoi(ms[4])
				if err != nil {
					fmt.Printf("ERROR: %s: %s\n", ms[4], err)
					return nil
				}
				duration := time.Duration(h)*time.Hour + time.Duration(min)*time.Minute + time.Duration(sec)*time.Second + time.Duration(nsec)*time.Nanosecond
				m.tags["duration"] = duration.String()
			}
		}
	case mkvparse.TitleElement:
		fmt.Printf("%s: %v\n", mkvparse.NameForElementID(id), s)
	default:
		fmt.Printf("%s: %v\n", mkvparse.NameForElementID(id), s)
	}
	//fmt.Printf("ELEMENT ID: %X\n", id)
	//fmt.Printf("STRING: %s\n", s)
	return nil
}
func (m *mkvHandler) HandleInteger(id mkvparse.ElementID, i int64, e mkvparse.ElementInfo) error {
	//fmt.Printf("INT: %d\n", i)
	return nil
}
func (m *mkvHandler) HandleFloat(id mkvparse.ElementID, f float64, e mkvparse.ElementInfo) error {
	//fmt.Printf("Float: %f\n", f)
	return nil
}
func (m *mkvHandler) HandleDate(id mkvparse.ElementID, t time.Time, e mkvparse.ElementInfo) error {
	fmt.Printf("Time: %s\n", t.Format(time.UnixDate))
	return nil
}
func (m *mkvHandler) HandleBinary(id mkvparse.ElementID, bs []byte, e mkvparse.ElementInfo) error {
	//fmt.Printf("BYTES: %d\n", len(bs))
	return nil
}
