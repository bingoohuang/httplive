package acl

import (
	"bufio"
	"encoding/csv"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingoohuang/sariaf"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/sirupsen/logrus"
)

// NewCasbin create a casbin object with model and policy string.
func NewCasbin(modelConf, policyConf string) (*casbin.Enforcer, error) {
	m, err := model.NewModelFromString(modelConf)
	if err != nil {
		return nil, err
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	ResetPolicyString(m, policyConf)
	e.AddFunction("timeAllow", func(args ...interface{}) (interface{}, error) {
		return TimeAllow(args[0].(string), args[1].(string)), nil
	})
	e.AddFunction("routerMatch", func(args ...interface{}) (interface{}, error) {
		return RouterMatch(args[0].(string), args[1].(string)), nil
	})
	e.AddFunction("wildMatch", func(args ...interface{}) (interface{}, error) {
		return WildcardMatch(args[0].(string), args[1].(string)), nil
	})

	return e, nil
}

// ResetPolicyString loads all policy rules from the string.
func ResetPolicyString(model model.Model, s string) {
	for _, v := range model {
		for _, ast := range v {
			ast.Policy = nil
			ast.PolicyMap = map[string]int{}
		}
	}

	for _, line := range SplitLines(s) {
		persist.LoadPolicyLine(line, model)
	}
}

// SplitLines split string s to lines, ignore lines start with #.
func SplitLines(s string) []string {
	buf := bufio.NewReader(strings.NewReader(s))
	lines := make([]string, 0)
	for {
		line, err := buf.ReadString('\n')
		if line == "" && err != nil {
			return lines
		}

		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		lines = append(lines, line)
	}
}

// CsvTokens spits line as CSV string.
func CsvTokens(line string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(line))
	r.Comma = ','
	r.Comment = '#'
	r.TrimLeadingSpace = true

	return r.Read()
}

// CasbinEpoch defines the start time of casbin.
var CasbinEpoch = time.Now()

// CasbinTimeLayout defines the time layout used in casbin.
const CasbinTimeLayout = "2006-01-02 15:04:05"

func RouterMatch(router, pattern string) bool {
	if pattern == "-" {
		return true
	}

	r := sariaf.New()
	if err := r.Handle(http.MethodGet, pattern, nil); err != nil {
		logrus.Errorf("failed to parse pattern %s: %v", pattern, err)
		return false
	}

	node, _ := r.Search(http.MethodGet, router)
	return node != nil
}

// TimeAllow 允许运行时间
// policy 格式
// 1. - 全部通过
// 2. 2020-12-31 00:00:00 截止到指定日期时间
// 3. 3d 自分配起3天内
// 4. 2020-12-31 00:00:00/2021-12-31 00:00:00 起始结束日期时间之内
// 5. 2020-12-31 00:00:00/3d 起始日期时间后的3天之内
func TimeAllow(request, policy string) bool {
	if policy == "-" {
		return true
	}

	if policy == "x" {
		return false
	}

	req, err := time.ParseInLocation(CasbinTimeLayout, request, time.Local)
	if err != nil {
		logrus.Errorf("failed to parse request %s: %v", request, err)
		return false
	}

	parts := strings.Split(policy, "/")
	if len(parts) == 1 {
		return timeUntil(CasbinTimeLayout, policy, policy, CasbinEpoch, req)
	}

	if len(parts) == 2 {
		p1 := parts[0]
		p2 := parts[1]

		from, err := time.ParseInLocation(CasbinTimeLayout, p1, time.Local)
		if err != nil {
			logrus.Errorf("unknown format of policy %s", policy)
		} else if req.Before(from) {
			return false
		}

		return timeUntil(CasbinTimeLayout, p2, policy, from, req)
	}

	logrus.Errorf("unknown format of policy %s", policy)
	return false
}

func timeUntil(layout, until, policy string, start, req time.Time) bool {
	// try until time
	if until, err := time.ParseInLocation(layout, until, time.Local); err == nil {
		return req.Before(until)
	}
	// try duration
	if d, err := time.ParseDuration(until); err == nil {
		return req.Before(start.Add(d))
	}

	logrus.Errorf("unknown format of policy %s", policy)
	return false
}

// WildcardMatch matches request with policy in wildcard mode.
// 1. - 全部通过
// 2. - a*/b* : a* or b*
func WildcardMatch(request, policy string) bool {
	if policy == "-" {
		return true
	}

	for _, p := range strings.Split(policy, "/") {
		if matched, err := filepath.Match(p, request); err != nil {
			logrus.Errorf("wildcardMatch pattern %s error %v", p, err)
		} else if matched {
			return true
		}
	}

	return false
}
