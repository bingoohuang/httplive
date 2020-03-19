package httplive

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	tests := []struct {
		method    string
		outExists bool
	}{
		{http.MethodOptions, true},
		{http.MethodGet, false},
		{http.MethodPost, false},
	}
	expectedHeader := []struct {
		key   string
		value string
	}{
		{"Access-Control-Allow-Origin", "*"},
		{"Access-Control-Max-Age", "86400"},
		{"Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE"},
		// nolint lll
		{"Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token"},
		{"Access-Control-Expose-Headers", "Content-Length"},
		{"Access-Control-Allow-Credentials", "true"},
	}
	f := CORSMiddleware()

	for _, tt := range tests {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req, err := http.NewRequest(tt.method, "/", nil)

		assert.Nil(t, err)

		c.Request = req

		f(c)

		for _, h := range expectedHeader {
			assert.Equal(t, h.value, c.Writer.Header().Get(h.key))
		}
	}
}
