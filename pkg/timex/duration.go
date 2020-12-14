package timex

import (
	"encoding/json"
	"fmt"
	"time"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' {
		sd := string(b[1 : len(b)-1])
		v, err := time.ParseDuration(sd)
		*d = Duration(v)
		return err
	}

	var id int64
	id, err = json.Number(b).Int64()
	// default unit to Milliseconds.
	*d = Duration(time.Duration(id) * time.Millisecond)

	return
}

func (d Duration) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%s"`, time.Duration(d).String())), nil
}
