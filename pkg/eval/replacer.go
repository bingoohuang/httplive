package eval

import (
	"regexp"
	"strings"
)

type Replacer struct {
	Items []ReplaceItem
}

type ReplaceItem struct {
	Reg *regexp.Regexp
	Val string
}

func ParseReplacer(expr string) (*Replacer, error) {
	items := make([]ReplaceItem, 0)
	for _, item := range strings.Fields(expr) {
		subs := strings.SplitN(item, "=>", 2)
		k, v := subs[0], subs[1]
		var kreg *regexp.Regexp
		if strings.HasSuffix(k, ",i") {
			kreg = regexp.MustCompile("(?i)" + k[:len(k)-2])
		} else {
			kreg = regexp.MustCompile(k)
		}

		items = append(items, ReplaceItem{Reg: kreg, Val: v})
	}

	return &Replacer{Items: items}, nil
}

func (r *Replacer) Replace(str string) string {
	s := str
	for _, item := range r.Items {
		s = item.Reg.ReplaceAllString(s, item.Val)
	}
	return s
}
