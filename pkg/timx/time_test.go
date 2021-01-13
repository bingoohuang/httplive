package timx

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeTruncate(t *testing.T) {
	testTime := time.Now()

	// this is wrong (outputs 2021-01-13T08:00:00+08:00)
	day := time.Hour * 24
	fmt.Println(testTime.Truncate(day).Format(time.RFC3339))
	_, offset := testTime.Zone()
	d := time.Duration(offset) * time.Second
	// // this is correct (outputs 2021-01-13T00:00:00+08:00)
	fmt.Println(testTime.Add(d).Truncate(day).Add(-d).Format(time.RFC3339))

	tt := Time(testTime)
	fmt.Println(tt.BeginningOfMonth())
	fmt.Println(tt.EndOfMonth())
	fmt.Println(tt.BeginningOfDay())
	fmt.Println(tt.EndOfDay())
}
