package timeago_test

import (
	"fmt"
	"time"

	"github.com/bingoohuang/httplive/pkg/timeago"
)

func parseTime(inputDate string) time.Time {
	layOut := "02/01/2006 15:04:05" // dd/mm/yyyy hh:mm:ss
	t, err := time.ParseInLocation(layOut, inputDate, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func ExampleFormat() {
	now := parseTime("06/05/2017 20:46:22")
	fmt.Println("The date time stamp now is : ", now)
	fmt.Println("02 March 1992 10:10 full=true is : ", timeago.Format(now, parseTime("02/03/1992 10:10:10"), true))
	fmt.Println("02 March 1992 10:10 full=false is : ", timeago.Format(now, parseTime("02/03/1992 10:10:10"), false))

	fmt.Println("06 May 2020 17:33 full=false is : ", timeago.Format(now, parseTime("06/05/2020 17:33:10"), false))
	fmt.Println("06 May 2020 17:33 full=true is : ", timeago.Format(now, parseTime("06/05/2020 17:33:10"), true))

	fmt.Println("06 May 2017 17:33 full=false is : ", timeago.Format(now, parseTime("06/05/2017 17:33:10"), false))
	fmt.Println("06 May 2017 17:33 full=true is : ", timeago.Format(now, parseTime("06/05/2017 17:33:10"), true))

	// Output:
	// The date time stamp now is :  2017-05-06 20:46:22 +0800 CST
	// 02 March 1992 10:10 full=true is :  25 years, 2 months, 4 days, 10 hours, 36 minutes, 12 seconds ago
	// 02 March 1992 10:10 full=false is :  25 years ago
	// 06 May 2020 17:33 full=false is :  3 years after
	// 06 May 2020 17:33 full=true is :  3 years, 3 hours, 13 minutes, 12 seconds after
	// 06 May 2017 17:33 full=false is :  3 hours ago
	// 06 May 2017 17:33 full=true is :  3 hours, 13 minutes, 12 seconds ago
}
