package process

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/man"
)

// code from https://github.com/m3ng9i/ran/blob/master/server/dirlist.go

type dirListFiles struct {
	Seq     int
	Name    string
	Url     string
	Size    string
	ModTime time.Time
}

type dirList struct {
	Title string
	Files []dirListFiles
}

const dirListTpl = `<!DOCTYPE HTML>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="initial-scale=1,width=device-width">
<title>{{.Title}}</title>
<style type="text/css">
body {
    background-color:white;
    color: #333333;
}
table {
    border-collapse: collapse;
}
table tr:nth-child(1) {
    background-color: #f0f0f0;
}
table th, table td {
    padding: 8px 10px;
    border:1px #dddddd solid;
    font-size: 14px;
}
table a {
    text-decoration: none;
}
table tr:hover {
    border:1px red solid;
}
table tr > td:nth-child(2), table tr > td:nth-child(3) {
    font-size: 13px;
}
</style>
</head>
<body>
<table>
<tr><th>#</th><th>Name</th><th>Size</th><th>Modification time</th></tr>
{{range $files := .Files}}
    <tr>
        <td>{{.Seq}}</td>
        <td><a href="{{.Url}}">{{.Name}}</a></td>
        <td>{{.Size}}</td>
        {{/* t2s example: {{ t2s .ModTime "2006-01-02 15:04"}} */}}
        <td>{{t2s .ModTime}}</td>
    </tr>
{{end}}
</table>
</body>
</html>`

var tplDirList = func() *template.Template {
	t, err := template.New("dirlist").
		Funcs(template.FuncMap{
			"t2s": timeToString,
		}).
		Parse(dirListTpl)
	if err != nil {
		log.Fatalf("Directory list template init error: %v", err)
	}
	return t
}()

func timeToString(t time.Time, format ...string) string {
	f := "2006-01-02 15:04:05"
	if len(format) > 0 && format[0] != "" {
		f = format[0]
	}
	return t.Format(f)
}

// ListDir lists content of a directory.
// If error occurs, this function will return an error and won't write anything to ResponseWriter.
func ListDir(w http.ResponseWriter, dir string, max int) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer iox.Close(f)

	info, err := f.Readdir(max)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	title := html.EscapeString(path.Base(dir))

	var files []dirListFiles

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
			dirListFiles{
				Seq:     n + 1,
				Name:    name,
				Url:     name,
				Size:    fmt.Sprintf("%d / %s", i.Size(), man.Bytes(uint64(i.Size()))),
				ModTime: i.ModTime(),
			},
		)
	}

	data := dirList{Title: title, Files: files}
	return tplDirList.Execute(w, data)
}
