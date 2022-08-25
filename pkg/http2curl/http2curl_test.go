package http2curl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func ExampleGetCurlCmd() {
	form := url.Values{}
	form.Add("age", "10")
	form.Add("name", "Hudson")
	body := form.Encode()

	payload := io.NopCloser(bytes.NewBufferString(body))
	req, _ := http.NewRequest(http.MethodPost, "http://foo.com/cats", payload)
	req.Header.Set("Api-Key", "123")

	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd)
	// Output:
	// curl -X POST -d 'age=10&name=Hudson' -H 'Api-Key: 123' 'http://foo.com/cats'
}

func ExampleGetCurlCmd_json() {
	payload := bytes.NewBufferString(`{"hello":"world","answer":42}`)
	req, _ := http.NewRequest("PUT", "http://a.b.c/abc?jlk=mno&pqr=stu", payload)
	req.Header.Set("Content-Type", "application/json")

	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd)
	// Output:
	// curl -X PUT -d '{"hello":"world","answer":42}' -H 'Content-Type: application/json' 'http://a.b.c/abc?jlk=mno&pqr=stu'
}

func ExampleGetCurlCmd_slice() {
	// See https://github.com/moul/http2curl/issues/12
	payload := bytes.NewBufferString(`{"hello":"world","answer":42}`)
	req, _ := http.NewRequest("PUT", "http://a.b.c/abc?jlk=mno&pqr=stu", payload)

	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd.Lines())

	// Output:
	// curl -X PUT \
	//   -d '{"hello":"world","answer":42}' \
	//   'http://a.b.c/abc?jlk=mno&pqr=stu'
}

func ExampleGetCurlCmd_noBody() {
	req, _ := http.NewRequest("PUT", "http://a.b.c/abc?jlk=mno&pqr=stu", nil)
	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd)
	// Output:
	// curl -X PUT 'http://a.b.c/abc?jlk=mno&pqr=stu'
}

func ExampleGetCurlCmd_emptyStringBody() {
	req, _ := http.NewRequest("PUT", "http://a.b.c/abc?jlk=mno&pqr=stu", bytes.NewBufferString(""))
	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd)
	// Output:
	// curl -X PUT 'http://a.b.c/abc?jlk=mno&pqr=stu'
}

func ExampleGetCurlCmd_newlineInBody() {
	req, _ := http.NewRequest("POST", "http://a.b.c/abc?jlk=mno&pqr=stu", bytes.NewBufferString("hello\nworld"))
	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd)
	// Output:
	// curl -X POST -d 'hello
	// world' 'http://a.b.c/abc?jlk=mno&pqr=stu'
}

func ExampleGetCurlCmd_specialCharsInBody() {
	payload := bytes.NewBufferString(`Hello $123 o'neill -"-`)
	req, _ := http.NewRequest("POST", "http://a.b.c/abc?jlk=mno&pqr=stu", payload)
	cmd, _ := GetCurlCmd(req)
	fmt.Println(cmd)
	// Output:
	// curl -X POST -d 'Hello $123 o'\''neill -"-' 'http://a.b.c/abc?jlk=mno&pqr=stu'
}

func ExampleGetCurlCmd_other() {
	payload := bytes.NewBufferString(`{"hello":"world","answer":42}`)
	req, err := http.NewRequest("PUT", "http://a.b.c/abc?jlk=mno&pqr=stu", payload)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-Auth-Token", "private-token")
	req.Header.Set("Content-Type", "application/json")

	cmd, err := GetCurlCmd(req)
	if err != nil {
		panic(err)
	}
	fmt.Println(cmd)
	// Output:
	// curl -X PUT -d '{"hello":"world","answer":42}' -H 'Content-Type: application/json' -H 'X-Auth-Token: private-token' 'http://a.b.c/abc?jlk=mno&pqr=stu'
}
