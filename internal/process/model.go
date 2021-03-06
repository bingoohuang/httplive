package process

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bingoohuang/gg/pkg/cast"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mssola/user_agent"

	"github.com/bingoohuang/httplive/pkg/acl"
	"github.com/bingoohuang/sariaf"
	"github.com/casbin/casbin/v2"
	"github.com/sirupsen/logrus"

	"github.com/bingoohuang/sysinfo"

	"github.com/bingoohuang/httplive/pkg/http2curl"

	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
)

// ContextKey as context key type.
type ContextKey int

// RouterResultKey as RouterResult key
const RouterResultKey ContextKey = iota

// ID is the ID for UnmarshalJSON from integer.
type ID string

// UnmarshalJSON unmarshals JSON from integer or string.
func (i *ID) UnmarshalJSON(b []byte) error {
	*i = ID(b)
	return nil
}

// Int convert ID to integer.
func (i ID) Int() int { return cast.ToInt(i) }

// APIDataModel ...
type APIDataModel struct {
	ID          ID     `json:"id" form:"id"`
	Endpoint    string `json:"endpoint" form:"endpoint"`
	Method      string `json:"method" form:"method"`
	MimeType    string `json:"mimeType"`
	Filename    string `json:"filename"`
	FileContent []byte `json:"-"`
	Body        string `json:"body"`

	dynamicValuers []DynamicValue
	ServeFn        gin.HandlerFunc `json:"-"`
}

// WsMessage ...
type WsMessage struct {
	Time           string            `json:"time"`
	Host           string            `json:"host"`
	Body           interface{}       `json:"body"`
	Response       json.RawMessage   `json:"response"`
	ResponseStatus int               `json:"status"`
	ResponseHeader map[string]string `json:"responseHeader"`
	ResponseSize   int               `json:"responseSize"`
	Header         map[string]string `json:"header"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query"`
	RemoteAddr     string            `json:"remoteAddr"`
}

// Endpoint is the structure for table httplive_endpoint.
type Endpoint struct {
	ID         ID     `name:"id"`
	Endpoint   string `name:"endpoint"`
	Methods    string `name:"methods"`
	MimeType   string `name:"mime_type"`
	Filename   string `name:"filename"`
	Body       string `name:"body"`
	CreateTime string `name:"create_time"`
	UpdateTime string `name:"update_time"`
	DeletedAt  string `name:"deleted_at"`
}

func (a APIDataModel) HandleFileDownload(c *gin.Context) {
	if !apiAuth(c) {
		return
	}

	rr := c.Request.Context().Value(RouterResultKey).(*RouterResult)
	rr.RouterServed = true
	rr.Filename = a.Filename
	c.Status(http.StatusOK)

	hl := c.Query("_hl")
	switch hl {
	case "conf":
		c.JSON(http.StatusOK, a)
		return
	}

	dl := c.Query("_dl")
	if dl == "" {
		http.ServeContent(c.Writer, c.Request, a.Filename, time.Now(), bytes.NewReader(a.FileContent))
		return
	}

	h := c.Header
	h("Content-Disposition", mime.FormatMediaType("attachment",
		map[string]string{"filename": a.Filename}))
	h("Content-Description", "File Transfer")
	h("Content-Type", "application/octet-stream")
	h("Content-Transfer-Encoding", "binary")
	h("Expires", "0")
	h("Cache-Control", "must-revalidate")
	h("Pragma", "public")
}

// JsTreeDataModel ...
type JsTreeDataModel struct {
	ID        int               `json:"id"`
	Key       string            `json:"key"`
	OriginKey string            `json:"originKey"`
	Text      string            `json:"text"`
	Type      string            `json:"type"`
	Children  []JsTreeDataModel `json:"children"`
}

func (a APIDataModel) getLabelByMethod() string {
	switch a.Method {
	case http.MethodGet:
		return "label label-primary label-small"
	case http.MethodPost:
		return "label label-success label-small"
	case http.MethodPut:
		return "label label-warning label-small"
	case http.MethodDelete:
		return "label label-danger label-small"
	default:
		return "label label-default label-small"
	}
}

func (a APIDataModel) CreateJsTreeModel() JsTreeDataModel {
	model := JsTreeDataModel{
		ID:        a.ID.Int(),
		OriginKey: util.JoinLowerKeys(a.Method, a.Endpoint),
		Key:       a.Endpoint,
		Text:      a.Endpoint,
		Children:  []JsTreeDataModel{},
	}

	model.Type = a.Method
	model.Text = fmt.Sprintf(`<span class="%v">%v</span> %v`, a.getLabelByMethod(), a.Method, a.Endpoint)

	return model
}

func (a APIDataModel) HandleJSON(c *gin.Context) {
	yes, fn := dealHl(c, a)
	if yes || a.ServeFn == nil {
		return
	}

	cw := util.NewGinCopyWriter(c.Writer)
	c.Writer = cw

	a.ServeFn(c)
	if fn != nil {
		fn(c)
	}

	rr := c.Request.Context().Value(RouterResultKey).(*RouterResult)
	if !rr.RouterServed {
		rr.RouterServed = true
		rr.RouterBody = cw.Bytes()
	}

	rr.RemoteAddr = c.Request.RemoteAddr
	rr.ResponseSize = cw.Size()
	rr.ResponseStatus = cw.Status()
	rr.ResponseHeader = util.ConvertHeader(cw.Header())
}

func (a *APIDataModel) InternalProcess(subRouter string) {
	acl.CasbinEpoch = time.Now()

	switch subRouter {
	case "/apiacl":
		a.apiacl()
	case "/adminacl":
		a.adminacl()
	}
}

var (
	authLock sync.Mutex

	apiCasbinEnforcer   *casbin.Enforcer
	adminCasbinEnforcer *casbin.Enforcer
	apiAuthHandler      func(c *gin.Context) (AuthResultType, string)
	adminAuthHandler    func(c *gin.Context) (AuthResultType, string)
)

func apiAuth(c *gin.Context) bool {
	authLock.Lock()
	defer authLock.Unlock()

	if apiAuthHandler == nil {
		return true
	}

	switch authResultType, user := apiAuthHandler(c); authResultType {
	case AuthResultIgnore:
	case AuthResultFailed:
		return true
	case AuthResultOK:
		ok, err := apiCasbinEnforcer.Enforce(user, c.Request.URL.Path, c.Request.Method, time.Now().Format(acl.CasbinTimeLayout))
		if err != nil {
			logrus.Warnf("failed to casbin %v", err)
		}

		if ok {
			return true
		}
	}

	c.Status(http.StatusForbidden)
	return false
}

type AuthResultType int

const (
	AuthResultIgnore AuthResultType = iota
	AuthResultOK
	AuthResultFailed
)

func (a *APIDataModel) adminacl() {
	e, _, authMap := a.createCasbin()

	authLock.Lock()
	defer authLock.Unlock()

	if e == nil {
		adminCasbinEnforcer = nil
		adminAuthHandler = nil
		return
	}

	adminCasbinEnforcer = e
	adminAuthHandler = func(c *gin.Context) (AuthResultType, string) {
		authHead := c.GetHeader("Authorization")
		if authHead == "" {
			return AuthResultOK, "anonymous"
		}

		user, ok := authMap[authHead]
		if !ok {
			realm := "Authorization Required"
			c.Header("WWW-Authenticate", "Basic realm="+strconv.Quote(realm))
			c.AbortWithStatus(http.StatusUnauthorized)
			return AuthResultFailed, ""
		}

		return AuthResultOK, user[:strings.Index(user, ":")]
	}
}

func (a *APIDataModel) apiacl() {
	e, sariafRouter, authMap := a.createCasbin()

	authLock.Lock()
	defer authLock.Unlock()

	if e == nil {
		apiCasbinEnforcer = nil
		apiAuthHandler = nil
		return
	}

	apiCasbinEnforcer = e
	apiAuthHandler = func(c *gin.Context) (AuthResultType, string) {
		node, _ := sariafRouter.Search(c.Request.Method, c.Request.URL.Path)
		if node == nil {
			return AuthResultIgnore, ""
		}

		authHead := c.GetHeader("Authorization")
		user, ok := authMap[authHead]
		if !ok {
			realm := "Authorization Required"
			c.Header("WWW-Authenticate", "Basic realm="+strconv.Quote(realm))
			c.AbortWithStatus(http.StatusUnauthorized)
			return AuthResultFailed, ""
		}

		return AuthResultOK, user[:strings.Index(user, ":")]
	}
}

func (a APIDataModel) createCasbin() (*casbin.Enforcer, *sariaf.Router, map[string]string) {
	modelConf := util.UnquoteCover(a.Body, "###START_MODEL###", "###END_MODEL###")
	policyConf := util.UnquoteCover(a.Body, "###START_POLICY###", "###END_POLICY###")
	authConf := util.UnquoteCover(a.Body, "###START_AUTH###", "###END_AUTH###")

	e, err := acl.NewCasbin(modelConf, policyConf)
	if err != nil {
		logrus.Warnf("failed to create casbin: %v", err)
		return nil, nil, nil
	}

	policyRows := e.GetNamedPolicy("p")
	sariafRouter := sariaf.New()
	for _, row := range policyRows {
		if err := sariafRouter.Handle(sariaf.MethodAny, row[1], nil); err != nil {
			logrus.Warnf("failed to create casbin: %v", err)
			return nil, nil, nil
		}
	}

	authMap := make(map[string]string)
	for _, row := range acl.SplitLines(authConf) {
		authHead := "Basic " + base64.StdEncoding.EncodeToString([]byte(row))
		authMap[authHead] = row
	}

	return e, sariafRouter, authMap
}

func (a *APIDataModel) TryDo(f func(m *APIDataModel)) {
	if a.ServeFn != nil {
		return
	}

	f(a)
}

func AdminAuth(c *gin.Context) {
	if adminAuthHandler == nil {
		return
	}

	authResultType, user := adminAuthHandler(c)
	if authResultType != AuthResultOK {
		return
	}

	ok, err := adminCasbinEnforcer.Enforce(user, c.Request.URL.Path, c.Request.Method, time.Now().Format(acl.CasbinTimeLayout))
	if err != nil {
		logrus.Warnf("failed to casbin %v", err)
	} else if !ok {
		authHead := c.GetHeader("Authorization")
		if authHead == "" {
			realm := "Authorization Required"
			c.Header("WWW-Authenticate", "Basic realm="+strconv.Quote(realm))
			c.AbortWithStatus(http.StatusUnauthorized)
		} else {
			c.AbortWithStatus(http.StatusForbidden)
		}
	}
}

func dealHl(c *gin.Context, ep APIDataModel) (bool, gin.HandlerFunc) {
	if !apiAuth(c) {
		return true, nil
	}

	ua := user_agent.New(c.Request.UserAgent())
	isBrowser := ua.OS() != ""
	useJSON := util.HasContentType(c.Request, "application/json") || !isBrowser
	hl := strings.ToLower(c.Query("_hl"))
	if strings.HasSuffix(hl, ".json") {
		useJSON = true
		hl = hl[:len(hl)-5]
	}

	switch hl {
	case "curl":
		values := c.Request.URL.Query()
		delete(values, "_hl")
		c.Request.URL.RawQuery = values.Encode()
		cmd, _ := http2curl.GetCurlCmd(c.Request)
		c.Data(http.StatusOK, util.ContentTypeText, []byte(cmd.String()))
	case "ip":
		ProcessIP(c, useJSON)
	case "echo":
		if useJSON {
			c.PureJSON(http.StatusOK, CreateRequestMap(c, &ep))
		} else {
			d, _ := httputil.DumpRequest(c.Request, true)
			c.Data(http.StatusOK, util.ContentTypeText, d)
		}
	case "time":
		if useJSON {
			c.PureJSON(http.StatusOK, gin.H{"time": util.TimeFmt(time.Now())})
		} else {
			c.Data(http.StatusOK, util.ContentTypeText, []byte(util.TimeFmt(time.Now())))
		}
	case "conf":
		util.GinData(c, []byte(ep.Body))
	case "sysinfo":
		showsMap := make(map[string]bool)
		for _, p := range strings.Split("host,mem,cpu,disk,interf,ps", ",") {
			showsMap[p] = true
		}
		if useJSON {
			c.PureJSON(http.StatusOK, sysinfo.GetSysInfo(showsMap))
		} else {
			c.Status(http.StatusOK)
			c.Header("Content-Type", util.ContentTypeText)
			sysinfo.PrintTable(showsMap, "~", c.Writer)
		}
	default:
		return dealMore(hl)
	}

	return true, nil
}

func dealMore(hl string) (bool, gin.HandlerFunc) {
	if strings.HasPrefix(hl, "sleep") {
		v := util.Or(hl[len("sleep"):], "1s")
		du, err := time.ParseDuration(v)
		if err != nil {
			du = 1 * time.Second
		}

		return false, func(c *gin.Context) { time.Sleep(du) }
	}

	return false, nil
}

func dynamicProcess(c *gin.Context, ep APIDataModel) bool {
	if len(ep.dynamicValuers) == 0 {
		return false
	}

	reqBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("E! readall %v", err)
		return false
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	for _, v := range ep.dynamicValuers {
		parameters := make(gin.H, len(v.ParametersEvaluator))
		for k, valuer := range v.ParametersEvaluator {
			parameters[k] = valuer(reqBody, c)
		}

		evaluateResult, err := v.Expr.Evaluate(parameters)
		if err != nil {
			log.Printf("E! Evaluate %s error %v", v.Expr.String(), err)
			return false
		}

		if yes, ok := evaluateResult.(bool); ok && yes {
			v.responseDynamic(ep, c)
			return true
		}
	}

	return false
}
