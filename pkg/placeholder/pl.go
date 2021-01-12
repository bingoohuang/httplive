package placeholder

import (
	"fmt"
	"regexp"
	"strings"
)

type Placeholder struct {
	Value string
	Vars  []interface{}
}

var varsRegexp = regexp.MustCompile(`#*\w+#`)

func Create(value string, vars map[string]interface{}, placeholder string) (*Placeholder, error) {
	plVars := make([]interface{}, 0)
	parsedQuery := Replace(value, vars)

	var err error

	parsedQuery = varsRegexp.ReplaceAllStringFunc(parsedQuery, func(s string) string {
		varName := s[1 : len(s)-1]
		varValue, ok := vars[varName]
		if !ok {
			err = fmt.Errorf("%s is not provided", varName)
		}

		plVars = append(plVars, varValue)
		return placeholder
	})

	if err != nil {
		return nil, err
	}

	return &Placeholder{Value: parsedQuery, Vars: plVars}, nil
}

func Replace(s string, vars map[string]interface{}) string {
	for k, v := range vars {
		k := fmt.Sprintf("{%s}", k)
		v := fmt.Sprintf("%v", v)
		s = strings.ReplaceAll(s, k, v)
	}

	return s
}
