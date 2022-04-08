package process

import (
	"fmt"
	"github.com/bingoohuang/gg/pkg/mathx"
	"github.com/bingoohuang/gg/pkg/ss"
	"html"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/man"
)

// code from https://github.com/m3ng9i/ran/blob/master/server/dirlist.go

type File struct {
	weight  int
	Seq     int
	Name    string
	Url     string
	Size    string
	ModTime time.Time
}

type DirList struct {
	Title string
	Files []File
}

// ListDir lists content of a directory.
// If error occurs, this function will return an error and won't write anything to ResponseWriter.
func ListDir(dir string, max int) (*DirList, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer iox.Close(f)

	info, err := f.Readdir(max)
	if err != nil {
		return nil, err
	}

	title := html.EscapeString(path.Base(dir))

	var files []File

	for n, i := range info {
		name := i.Name()
		if i.IsDir() {
			name += "/"
		}

		// skip hidden path
		if strings.HasPrefix(name, ".") {
			continue
		}

		files = append(files,
			File{
				weight:  ss.Ifi(i.IsDir(), 0, 1),
				Seq:     n + 1,
				Name:    name,
				Url:     name,
				Size:    fmt.Sprintf("%d / %s", i.Size(), man.Bytes(uint64(i.Size()))),
				ModTime: i.ModTime(),
			},
		)
	}

	sort.SliceStable(files, func(i, j int) bool {
		ii, jj := files[i], files[j]
		if ii.weight != jj.weight {
			return ii.weight < jj.weight
		}

		return fileNameLess(ii, jj)
	})

	for i := range files {
		files[i].Seq = i + 1
	}

	return &DirList{Title: title, Files: files}, nil
}

var numReg = regexp.MustCompile(`\d+`)

func fileNameLess(a, b File) bool {
	na := numReg.FindAllString(a.Name, -1)
	nb := numReg.FindAllString(b.Name, -1)

	l := mathx.Min(len(na), len(nb))
	for i := 0; i < l; i++ {
		ia := ss.ParseInt(strings.TrimLeft(na[i], "0"))
		ib := ss.ParseInt(strings.TrimLeft(nb[i], "0"))
		if ia == ib {
			continue
		}

		return ia < ib
	}

	if !a.ModTime.Equal(b.ModTime) {
		return a.ModTime.After(b.ModTime)
	}

	return a.Size < b.Size
}
