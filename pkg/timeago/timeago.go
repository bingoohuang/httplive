package timeago

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Format Go date time in a pretty way. ex : just now, a minute ago, 2 hours ago , 3 minutes ago
// https://socketloop.com/tutorials/golang-human-readable-time-elapsed-format-such-as-5-days-ago
// https://github.com/andanhm/go-prettytime
// https://github.com/search?l=Go&q=time+ago&type=Repositories
// https://www.npmjs.com/package/javascript-time-ago
func Format(now, then time.Time, full bool) string {
	var parts []string
	var text string

	year2, month2, day2 := now.Date()
	hour2, minute2, second2 := now.Clock()

	year1, month1, day1 := then.Date()
	hour1, minute1, second1 := then.Clock()

	year := math.Abs(float64(int(year2 - year1)))
	month := math.Abs(float64(int(month2 - month1)))
	day := math.Abs(float64(int(day2 - day1)))
	hour := math.Abs(float64(int(hour2 - hour1)))
	minute := math.Abs(float64(int(minute2 - minute1)))
	second := math.Abs(float64(int(second2 - second1)))

	week := math.Floor(day / 7)
	if now.After(then) {
		text = " ago"
	} else {
		text = " after"
	}

	if year > 0 {
		parts = append(parts, strconv.Itoa(int(year))+" year"+s(year))
	}

	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if month > 0 {
		parts = append(parts, strconv.Itoa(int(month))+" month"+s(month))
	}
	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if week > 0 {
		parts = append(parts, strconv.Itoa(int(week))+" week"+s(week))
	}
	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if day > 0 {
		parts = append(parts, strconv.Itoa(int(day))+" day"+s(day))
	}
	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if hour > 0 {
		parts = append(parts, strconv.Itoa(int(hour))+" hour"+s(hour))
	}
	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if minute > 0 {
		parts = append(parts, strconv.Itoa(int(minute))+" minute"+s(minute))
	}
	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if second > 0 {
		parts = append(parts, strconv.Itoa(int(second))+" second"+s(second))
	}
	if !full && len(parts) > 0 {
		return parts[0] + text
	}

	if len(parts) == 0 {
		return "just now"
	}

	return strings.Join(parts, ", ") + text
}

func s(x float64) string {
	if int(x) == 1 {
		return ""
	}
	return "s"
}
