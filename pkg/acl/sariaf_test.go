package acl_test

import (
	"github.com/bingoohuang/httplive/pkg/acl"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestRouter(t *testing.T) {
	router := acl.NewRouter()
	router.Handle(http.MethodGet, "/", "a")
	router.Handle(http.MethodGet, "/posts", nil)
	router.Handle(http.MethodGet, "/posts/*id", "id")
	router.Handle(http.MethodPost, "/posts", nil)
	router.Handle(http.MethodPatch, "/posts/:id", nil)
	router.Handle(http.MethodPut, "/posts/:id", nil)
	router.Handle(http.MethodDelete, "/posts/:id", nil)
	router.Handle(http.MethodGet, "/error", nil)
	router.Handle(acl.MethodAny, "/any", nil)

	yes, params, tag := router.Search(http.MethodGet, "/")
	assert.True(t, yes)
	assert.Equal(t, acl.Params{}, params)
	assert.Equal(t, "a", tag)

	yes, params, tag = router.Search(http.MethodGet, "/posts")
	assert.True(t, yes)
	assert.Equal(t, acl.Params{}, params)
	assert.Nil(t, tag)

	yes, params, tag = router.Search(http.MethodGet, "/posts/123")
	assert.True(t, yes)
	assert.Equal(t, acl.Params{"id": "123"}, params)
	assert.Equal(t, "id", tag)

	yes, params, tag = router.Search(http.MethodGet, "/posts/123/456")
	assert.True(t, yes)
	assert.Equal(t, acl.Params{"id": "123/456"}, params)
	assert.Equal(t, "id", tag)

	yes, params, tag = router.Search(http.MethodGet, "/others")
	assert.False(t, yes)
	assert.Nil(t, params)
	assert.Nil(t, tag)

	yes, params, tag = router.Search(http.MethodGet, "/any")
	assert.True(t, yes)

	yes, params, tag = router.Search(http.MethodPost, "/any")
	assert.True(t, yes)
}
