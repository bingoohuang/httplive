package http2curl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/bingoohuang/httplive/pkg/util"
)

// CurlCmd contains exec.Cmd compatible slice + helpers
type CurlCmd []string

func (c *CurlCmd) append(item string) { *c = append(*c, item) }

// Lines returns a ready to copy/paste cmd
func (c CurlCmd) Lines() string { return strings.Join(c, " \\\n  ") }

// String returns a ready to copy/paste cmd
func (c CurlCmd) String() string { return strings.Join(c, " ") }

// nopCloser is used to create a new io.ReadCloser for req.Body
type nopCloser struct{ io.Reader }

func (nopCloser) Close() error { return nil }

func bashEscape(s string) string { return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'` }

// GetCurlCmd returns a CurlCmd corresponding to a http.Request
func GetCurlCmd(r *http.Request) (*CurlCmd, error) {
	c := &CurlCmd{}
	c.append("curl -X " + r.Method)

	if err := c.appendBody(r); err != nil {
		return nil, err
	}

	c.appendHeaders(r)
	c.append(bashEscape(createURL(r)))

	return c, nil
}

func createURL(r *http.Request) string {
	u := *r.URL
	u.Scheme = util.Or(u.Scheme, "http")
	u.Host = util.Or(u.Host, r.Host)

	return u.String()
}

func (c *CurlCmd) appendHeaders(r *http.Request) {
	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h := fmt.Sprintf("%s: %s", k, strings.Join(r.Header[k], " "))
		c.append("-H " + bashEscape(h))
	}
}

func (c *CurlCmd) appendBody(r *http.Request) error {
	if r.Body == nil {
		return nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) > 0 {
		c.append("-d " + bashEscape(string(body)))
	}

	r.Body = nopCloser{bytes.NewBuffer(body)}
	return nil
}
