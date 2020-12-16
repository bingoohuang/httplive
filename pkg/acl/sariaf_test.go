package acl_test

import (
	"github.com/bingoohuang/httplive/pkg/acl"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestRouter(t *testing.T) {
	router := acl.NewRouter()
	assert.Nil(t, router.Handle(acl.MethodAny, "/any", nil))
	assertSearch(t, router, http.MethodGet, "/any", true, acl.Params{}, nil)

	assert.Nil(t, router.Handle(http.MethodGet, "/", "a"))
	assert.Equal(t, acl.ErrDuplicateRouter, router.Handle(http.MethodGet, "/", "a"))
	assert.Nil(t, router.Handle(http.MethodGet, "/posts", nil))
	assert.Nil(t, router.Handle(http.MethodGet, "/posts/*id", "idstar"))
	assert.Equal(t, acl.ErrRouterSyntax, router.Handle(http.MethodGet, "/posts/*id/*xx", nil))
	assert.Equal(t, acl.ErrDuplicateRouter, router.Handle(http.MethodGet, "/posts/*id", "idstar"))
	assert.Nil(t, router.Handle(http.MethodPost, "/posts", nil))
	assert.Nil(t, router.Handle(http.MethodPatch, "/posts/:id", nil))
	assert.Nil(t, router.Handle(http.MethodPut, "/posts/:id", nil))
	assert.Equal(t, acl.ErrDuplicateRouter, router.Handle(http.MethodPut, "/posts/:id", nil))
	assert.Nil(t, router.Handle(http.MethodPut, "/posts/:id/:name", nil))
	assert.Equal(t, acl.ErrDuplicateRouter, router.Handle(http.MethodPut, "/posts/:id:name", nil))
	assert.Nil(t, router.Handle(http.MethodDelete, "/posts/:id", nil))
	assert.Nil(t, router.Handle(http.MethodGet, "/error", nil))

	assertSearch(t, router, http.MethodGet, "/", true, acl.Params{}, "a")
	assertSearch(t, router, http.MethodGet, "/posts", true, acl.Params{}, nil)
	assertSearch(t, router, http.MethodGet, "/posts/123", true, acl.Params{"id": "123"}, "idstar")
	assertSearch(t, router, http.MethodGet, "/posts/123/456", true, acl.Params{"id": "123/456"}, "idstar")
	assertSearch(t, router, http.MethodGet, "/others", false, nil, nil)
	assertSearch(t, router, http.MethodGet, "/any", true, acl.Params{}, nil)
	assertSearch(t, router, http.MethodPost, "/any", true, acl.Params{}, nil)
}

func assertSearch(t *testing.T, router *acl.Router, method, routerPath string,
	expectedFound bool, expectedParam acl.Params, expectedTag interface{}) {
	yes, params, tag := router.Search(method, routerPath)
	assert.Equal(t, expectedFound, yes)
	assert.Equal(t, expectedParam, params)
	assert.Equal(t, expectedTag, tag)
}
