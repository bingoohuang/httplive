package httplive

import (
	"net/http"
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
		{http.MethodPost, "foo", "postfoo"},
		{http.MethodPost, "FOO", "postfoo"},
		{http.MethodPost, "foo", "postfoo"},
		{http.MethodPost, "FOO", "postfoo"},
		{"ÄËÏ", "ÖÜ", "äëïöü"},
		{http.MethodPost, "///", "post///"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.out, CreateEndpointKey(tt.method, tt.endpoint))
	}
}
