package timx

import "time"

type Time time.Time

// TruncateTime returns the the truncated time by d.
// this function fix the location problem for the direct time.Truncate method.
// eg. if d = 1d, the result will be 2021-01-13T00:00:00+08:00.
func (t Time) TruncateTime(d time.Duration) time.Time {
	tt := time.Time(t)
	_, offset := tt.Zone()
	fix := time.Duration(offset) * time.Second
	return tt.Add(fix).Truncate(d).Add(-fix)
}

// BeginningOfMonth returns beginning of month like 2021-01-01 00:00:00 +0800 CST.
func (t Time) BeginningOfMonth() time.Time {
	tt := time.Time(t)
	return time.Date(tt.Year(), tt.Month(), 1, 0, 0, 0, 0, tt.Location())
}

// BeginningOfNextMonth returns beginning of next month like 2021-01-01 00:00:00 +0800 CST.
func (t Time) BeginningOfNextMonth() time.Time {
	return t.BeginningOfMonth().AddDate(0, 1, 0)
}

// EndOfMonth returns end of month like 2021-01-31 23:59:59 +0800 CST.
func (t Time) EndOfMonth() time.Time {
	return t.BeginningOfMonth().AddDate(0, 1, 0).Add(-time.Second)
}

// BeginningOfDay returns the beginning of day like 2021-01-13 00:00:00 +0800 CST.
func (t Time) BeginningOfDay() time.Time {
	tt := time.Time(t)
	return time.Date(tt.Year(), tt.Month(), tt.Day(), 0, 0, 0, 0, tt.Location())
}

// BeginningOfNextDay returns the beginning of day like 2021-01-13 00:00:00 +0800 CST.
func (t Time) BeginningOfNextDay() time.Time {
	return t.BeginningOfDay().AddDate(0, 0, 1)
}

// EndOfDay returns end of day like 2021-01-13 23:59:59 +0800 CST.
func (t Time) EndOfDay() time.Time {
	return t.BeginningOfDay().AddDate(0, 0, 1).Add(-time.Second)
}
