package process

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/man"
	"github.com/bingoohuang/gg/pkg/mathx"
	"github.com/bingoohuang/gg/pkg/ss"
)

// code from https://github.com/m3ng9i/ran/blob/master/server/dirlist.go

type File struct {
	ModTime   time.Time
	Name      string
	Url       string
	DirFiles  string
	HumanSize string
	Md5sum    string
	weight    int
	Seq       int
	Size      int64
}

type DirList struct {
	Title string
	Files []File
}

func DirSize(path string) (size, files int64, err error) {
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
			files++
		}
		return err
	})
	return
}

// ListDir lists content of a directory.
// If error occurs, this function will return an error and won't write anything to ResponseWriter.
func ListDir(dir, rawQuery string, max int) (*DirList, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer iox.Close(f)

	info, err := f.Readdir(max)
	if err != nil {
		if errors.Is(err, io.EOF) { // blank directory
			return &DirList{}, nil
		}
		return nil, err
	}

	title := html.EscapeString(filepath.Base(dir))

	var files []File

	if rawQuery != "" {
		rawQuery = "?" + rawQuery
	}

	for n, i := range info {
		name := i.Name()
		if strings.HasPrefix(name, ".") { // skip hidden path
			continue
		}

		size := i.Size()
		dirFiles := "-"
		if i.IsDir() {
			name += "/"
			var totalFiles int64
			size, totalFiles, _ = DirSize(filepath.Join(dir, name))
			dirFiles = fmt.Sprintf("%d", totalFiles)
		}

		file := File{
			weight:    ss.Ifi(i.IsDir(), 0, 1),
			Seq:       n + 1,
			Name:      name,
			Url:       name,
			Size:      size,
			HumanSize: man.Bytes(uint64(size)),
			DirFiles:  dirFiles,
			ModTime:   i.ModTime(),
		}
		if i.IsDir() {
			file.Url += rawQuery
		} else {
			file.Md5sum = md5sumFile(filepath.Join(dir, i.Name()))
		}
		files = append(files, file)
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

func md5sumFile(name string) string {
	file, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer file.Close()

	hash := md5.New()
	if _, err = io.Copy(hash, file); err != nil {
		return ""
	}

	return hex.EncodeToString(hash.Sum(nil))
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
