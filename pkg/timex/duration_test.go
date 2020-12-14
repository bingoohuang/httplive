package timex_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bingoohuang/httplive/pkg/timex"
)

type Ex struct {
	S timex.Duration
	I timex.Duration
}

func ExampleMarshal() {
	var ex Ex
	in := strings.NewReader(`{"S": "15s350ms", "I": 4000}`)
	err := json.NewDecoder(in).Decode(&ex)
	if err != nil {
		panic(err)
	}

	fmt.Println("Decoded:", ex)

	out, err := json.Marshal(ex)
	if err != nil {
		panic(err)
	}

	fmt.Println("Encoded:", string(out))

	// Output:
	// Decoded: {15350000000 4000000000}
	// Encoded: {"S":"15.35s","I":"4s"}
}
