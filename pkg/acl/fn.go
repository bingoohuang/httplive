package acl

import (
	"bufio"
	"encoding/csv"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/sirupsen/logrus"
)

// ResetPolicyString loads all policy rules from the string.
func ResetPolicyString(model model.Model, s string) error {
	for _, v := range model {
		for _, ast := range v {
			ast.Policy = nil
			ast.PolicyMap = map[string]int{}
		}
	}

	buf := bufio.NewReader(strings.NewReader(s))
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		persist.LoadPolicyLine(line, model)
	}
}

// SplitLines split string s to lines
func SplitLines(s string) []string {
	buf := bufio.NewReader(strings.NewReader(s))
	lines := make([]string, 0)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			return lines
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lines = append(lines, line)
	}

	return lines
}

// CsvTokens spits line as CSV string.
func CsvTokens(line string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(line))
	r.Comma = ','
	r.Comment = '#'
	r.TrimLeadingSpace = true

	return r.Read()
}

// CasbinStartTime defines the start time of casbin.
var CasbinStartTime = time.Now()

// CasbinTimeLayout defines the time layout used in casbin.
const CasbinTimeLayout = "2006-01-02 15:04:05"

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

	req, err := time.Parse(CasbinTimeLayout, request)
	if err != nil {
		logrus.Errorf("failed to parse request %s: %v", request, err)
		return false
	}

	parts := strings.Split(policy, "/")
	if len(parts) == 1 {
		return timeUntil(CasbinTimeLayout, policy, policy, CasbinStartTime, req)
	}

	if len(parts) == 2 {
		p1 := parts[0]
		p2 := parts[1]

		from, err := time.Parse(CasbinTimeLayout, p1)
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
	if until, err := time.Parse(layout, until); err == nil {
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
func WildcardMatch(request, policy string) bool {
	for _, p := range strings.Split(policy, "/") {
		if matched, err := filepath.Match(p, request); err != nil {
			logrus.Errorf("wildcardMatch pattern %s error %v", p, err)
		} else if matched {
			return true
		}
	}

	return false
}
