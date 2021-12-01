package process

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bingoohuang/gg/pkg/thinktime"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/bingoohuang/jj"
	"github.com/gin-gonic/gin"
)

// MockbinCookie defines the cookie format.
type MockbinCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	MaxAge   int    `json:"maxAge"`
	Path     string `json:"path"`
	Domain   string `json:"domain"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
	SameSite string `json:"sameSite"`
}

func (v MockbinCookie) SetCookie(c *gin.Context) {
	c.SetSameSite(parseSameSite(v.SameSite)) // must called before c.SetCookie()
	c.SetCookie(v.Name, v.Value, v.MaxAge, v.Path, v.Domain, v.Secure, v.HTTPOnly)
}

func parseSameSite(sameSite string) http.SameSite {
	switch lowerVal := strings.ToLower(sameSite); lowerVal {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}

// Mockbin defines the mockbin struct.
type Mockbin struct {
	Status      int               `json:"status"`
	Method      string            `json:"method"`
	RedirectURL string            `json:"redirectURL"`
	Headers     map[string]string `json:"headers"`
	Cookies     []MockbinCookie   `json:"cookies"`
	Close       bool              `json:"close"`
	ContentType string            `json:"contentType"`
	Payload     json.RawMessage   `json:"payload"`
	Sleep       string            `json:"sleep"`
}

func countIf(cond bool) int {
	if cond {
		return 1
	}

	return 0
}

// IsValid tells the mockbin is valid or not.
func (m Mockbin) IsValid() bool {
	return countIf(m.Status >= 100)+
		countIf(m.Method != "")+
		countIf(m.RedirectURL != "")+
		countIf(m.ContentType != "")+
		countIf(len(m.Payload) > 0)+
		countIf(len(m.Headers) > 0)+
		countIf(len(m.Cookies) > 0) >= 1
}

func (m Mockbin) Redirect(c *gin.Context) {
	switch m.Status {
	// 301 Moved Permanently: 请求的资源已永久移动到新位置，并且将来任何对此资源的引用都应该使用本响应返回的若干个URI之一
	// 302 Found: 请求的资源现在临时从不同的URI响应请求。由于这样的重定向是临时的，客户端应当继续向原有地址发送以后的请求,
	// HTTP 1.0中的意义是Moved Temporarily,但是很多浏览器的实现是按照303的处实现的，
	// 所以HTTP 1.1中增加了 303和307的状态码来区分不同的行为
	// 303 See Other (since HTTP/1.1): 对应当前请求的响应可以在另一个URI上被找到，而且客户端应当采用GET的方式访问那个资源
	// 304 Not Modified (RFC 7232): 请求的资源没有改变
	// 305 Use Proxy (since HTTP/1.1): 被请求的资源必须通过指定的代理才能被访问
	// 306 Switch Proxy: 在最新版的规范中，306状态码已经不再被使用
	// 307 Temporary Redirect (since HTTP/1.1): 请求的资源现在临时从不同的URI响应请求,和303不同，它还是使用原先的Method
	// 308 Permanent Redirect (RFC 7538): 请求的资源已永久移动到新位置,并且新请求的Method不能改变
	case 301, 302, 303, 307, 308:
		c.Redirect(m.Status, m.RedirectURL)
	default:
		c.Redirect(302, m.RedirectURL)
	}
}

func (m Mockbin) Handle(c *gin.Context) {
	M := strings.ToUpper(m.Method)
	if M != "" && !strings.Contains(M, "ANY") && !strings.Contains(M, c.Request.Method) {
		c.Status(http.StatusMethodNotAllowed)
		return
	}

	for k, v := range m.Headers {
		c.Header(k, v)
	}

	for _, v := range m.Cookies {
		v.Path = util.Or(v.Path, "/")
		v.SetCookie(c)
	}

	if m.Close {
		c.Header("Connection", "close")
	}

	if m.RedirectURL != "" {
		m.Redirect(c)
		return
	}

	if m.ContentType == "" {
		m.ContentType = util.DetectContentType(m.Payload)
	}

	if m.Sleep != "" {
		thinkTime, _ := thinktime.ParseThinkTime(m.Sleep)
		if thinkTime != nil {
			thinkTime.Think(true)
		}
	}

	payload := string(m.Payload)
	if jj.Valid(payload) {
		payload = jj.Gen(payload)
	}

	c.Data(m.Status, m.ContentType, []byte(payload))
}
