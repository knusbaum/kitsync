package meta

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/abema/go-mp4"
	"github.com/dhowden/tag"
	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	"github.com/gabriel-vasile/mimetype"
)

func GetTags(fname string) map[string]string {
	tags := make(map[string]string)
	if mtags, err := getMediaTags(fname); err == nil {
		MergeTags(tags, mtags)
	}
	// Need to detect when EXIF is likely, because this operation is expensive (it scans the full file)
	// 	if exifTags, err := getEXIFTags(fname); err == nil {
	// 		merge(tags, exifTags)
	// 	}
	if mp4Tags, err := getMP4Tags(fname); err == nil {
		MergeTags(tags, mp4Tags)
	}
	if mkvTags, err := getMKVTags(fname); err == nil {
		MergeTags(tags, mkvTags)
	}
	if mime, _ := detectMIME(fname); mime != "" {
		tags["mime"] = mime
	}
	tags["added"] = time.Now().Format(time.UnixDate)
	if ext := path.Ext(fname); ext != "" {
		tags["file_extension"] = ext
	}
	if base := path.Base(fname); base != "" {
		tags["file_name"] = base
	}
	return tags
}

// MergeTags adds all the tags in src to dst. Tags already present in dst are overwritten.
func MergeTags(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

func detectMIME(fname string) (string, error) {
	mtype, err := mimetype.DetectFile(fname)
	if err != nil {
		return "", err
	}
	return mtype.String(), nil
}

func getMP4Tags(fname string) (map[string]string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tags := make(map[string]string)
	info, err := mp4.Probe(f)
	if err != nil {
		return nil, err
	}
	if info.Duration == 0 {
		// Assume info is blank.
		return nil, nil
	}
	// 	tags["mp4.major_brand"] = string(info.MajorBrand[:])
	// 	tags["mp4.minor_version"] = fmt.Sprintf("%d", info.MinorVersion)
	// 	var cbrands []string
	// 	for _, brand := range info.CompatibleBrands {
	// 		cbrands = append(cbrands, string(brand[:]))
	// 	}
	// 	tags["mp4.compatible_brands"] = strings.Join(cbrands, ",")
	// 	tags["mp4.fast_start"] = fmt.Sprintf("%t", info.FastStart)
	// 	tags["mp4.timescale"] = fmt.Sprintf("%d", info.Timescale)
	tags["duration"] = time.Duration(info.Duration * uint64(info.Timescale) * uint64(time.Microsecond)).String()
	return tags, nil
}

func getEXIFTags(fname string) (map[string]string, error) {
	ex, err := exif.SearchFileAndExtractExif(fname)
	if err != nil {
		fmt.Printf("EXIF: %s\n", err)
		return nil, err
	}
	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		fmt.Printf("EXIF: %s\n", err)
		return nil, err
	}

	ti := exif.NewTagIndex()

	_, index, err := exif.Collect(im, ti, ex)
	if err != nil {
		fmt.Printf("EXIF: %s\n", err)
		return nil, err
	}
	root := index.RootIfd
	tags := make(map[string]string)
	for _, t := range root.DumpTags() {
		// 		fmt.Printf("%v\n", t)
		// 		fmt.Printf("\tPath: %s\n", t.IfdPath())
		// 		fmt.Printf("\tString: %s\n", t.String())
		// 		fmt.Printf("\tTagName: %s\n", t.TagName())
		// 		fmt.Printf("\tTagType: %v\n", t.TagType())
		// 		if f, err := t.Format(); err == nil {
		// 			fmt.Printf("\t%s\n", f)
		// 		}
		if t.TagType() == exifcommon.TypeUndefined {
			bs, _ := t.GetRawBytes()
			f, _ := t.Format()
			fmt.Printf("%s: %d bytes (%s) (%s)\n", t.TagName(), t.UnitCount(), string(bs), f)
		} else if f, err := t.FormatFirst(); err == nil {
			fmt.Printf("%s: %s\n", t.TagName(), f)
			tags["exif."+t.TagName()] = f
		}
	}
	// 	for _, ifd := range index.Ifds {
	// 		fmt.Printf("FOUND IFD: %#v\n", ifd)
	// 	}
	//fmt.Printf("EXIF: FOUND NOTHING.\n")
	return tags, nil
}

func getMediaTags(fname string) (map[string]string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string)

	other := m.Raw()
	for k, v := range other {
		//tags["media."+k] = fmt.Sprintf("%v", v)
		//fmt.Printf("TAG: %v -> %v\n", k, v)
		if k == "TRCK" {
			tags["media.track"] = fmt.Sprintf("%v", v)
		}
	}

	if v := m.Format(); v != "" {
		tags["media.format"] = string(v)
	}
	if v := m.FileType(); v != "" {
		tags["media.file_type"] = string(v)
	}
	if v := m.Title(); v != "" {
		tags["media.title"] = v
	}
	if v := m.Album(); v != "" {
		tags["media.album"] = v
	}
	if v := m.Artist(); v != "" {
		tags["media.artist"] = v
	}
	if v := m.AlbumArtist(); v != "" {
		tags["media.album_artist"] = v
	}
	if v := m.Composer(); v != "" {
		tags["media.composer"] = v
	}
	if v := m.Year(); v > 0 {
		tags["media.year"] = fmt.Sprintf("%d", v)
	}
	if v := m.Genre(); v != "" {
		tags["media.genre"] = v
	}
	if v, t := m.Track(); t > 0 {
		tags["media.track"] = fmt.Sprintf("%d/%d", v, t)
	}
	if v, t := m.Disc(); t > 0 {
		tags["media.disc"] = fmt.Sprintf("%d/%d", v, t)
	}
	if v := m.Picture(); v != nil {
		tags["media.picture.ext"] = v.Ext
		tags["media.picture.mime_type"] = v.MIMEType
		tags["media.picture.type"] = v.Type
		tags["media.picture.description"] = v.Description
	}
	// Skip lyrics for now. They're too big for a tag
	// 	if v := m.Lyrics(); v != "" {
	// 		tags["media.lyrics"] = v
	// 	}
	if v := m.Comment(); v != "" {
		tags["media.comment"] = v
	}

	return tags, nil
}
