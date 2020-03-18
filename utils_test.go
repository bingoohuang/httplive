package httplive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateEndpointKey(t *testing.T) {
	tests := []struct {
		method   string
		endpoint string
		out      string
	}{
		{"", "", ""},
		{"post", "foo", "postfoo"},
		{"post", "FOO", "postfoo"},
		{"POST", "foo", "postfoo"},
		{"POST", "FOO", "postfoo"},
		{"ÄËÏ", "ÖÜ", "äëïöü"},
		{"POST", "///", "post///"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.out, CreateEndpointKey(tt.method, tt.endpoint))
	}
}
