package fastrouter_test

import (
	"net/http"
	"testing"

	"github.com/bingoohuang/httplive/pkg/fastrouter"
	"github.com/stretchr/testify/assert"
)

func TestRouter(t *testing.T) {
	r := fastrouter.NewRouter()
	assert.Nil(t, r.Handle(fastrouter.MethodAny, "/any", nil))
	assertSearch(t, r, http.MethodGet, "/any", true, fastrouter.Params{}, nil)

	assert.Nil(t, r.Handle(http.MethodGet, "/", "a"))
	assert.Equal(t, fastrouter.ErrDuplicateRouter, r.Handle(http.MethodGet, "/", "a"))
	assert.Nil(t, r.Handle(http.MethodGet, "/posts", nil))
	assert.Nil(t, r.Handle(http.MethodGet, "/posts/*id", "idstar"))
	assert.Equal(t, fastrouter.ErrRouterSyntax, r.Handle(http.MethodGet, "/posts/*id/*xx", nil))
	assert.Equal(t, fastrouter.ErrDuplicateRouter, r.Handle(http.MethodGet, "/posts/*id", "idstar"))
	assert.Nil(t, r.Handle(http.MethodPost, "/posts", nil))
	assert.Nil(t, r.Handle(http.MethodPatch, "/posts/:id", nil))
	assert.Nil(t, r.Handle(http.MethodPut, "/posts/:id", nil))
	assert.Equal(t, fastrouter.ErrDuplicateRouter, r.Handle(http.MethodPut, "/posts/:id", nil))
	assert.Nil(t, r.Handle(http.MethodPut, "/posts/:id/:name", nil))
	assert.Equal(t, fastrouter.ErrDuplicateRouter, r.Handle(http.MethodPut, "/posts/:id:name", nil))
	assert.Nil(t, r.Handle(http.MethodDelete, "/posts/:id", nil))
	assert.Nil(t, r.Handle(http.MethodGet, "/error", nil))

	assertSearch(t, r, http.MethodGet, "/", true, fastrouter.Params{}, "a")
	assertSearch(t, r, http.MethodGet, "/posts", true, fastrouter.Params{}, nil)
	assertSearch(t, r, http.MethodGet, "/posts/123", true, fastrouter.Params{"id": "123"}, "idstar")
	assertSearch(t, r, http.MethodGet, "/posts/123/456", true, fastrouter.Params{"id": "123/456"}, "idstar")
	assertSearch(t, r, http.MethodGet, "/others", false, nil, nil)
	assertSearch(t, r, http.MethodGet, "/any", true, fastrouter.Params{}, nil)
	assertSearch(t, r, http.MethodPost, "/any", true, fastrouter.Params{}, nil)
}

func assertSearch(t *testing.T, router *fastrouter.Router, method, routerPath string,
	expectedFound bool, expectedParam fastrouter.Params, expectedTag interface{}) {
	yes, params, tag := router.Search(method, routerPath)
	assert.Equal(t, expectedFound, yes)
	assert.Equal(t, expectedParam, params)
	assert.Equal(t, expectedTag, tag)
}
