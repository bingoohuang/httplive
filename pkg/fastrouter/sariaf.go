package fastrouter

import (
	"errors"
	"strings"
)

var (
	ErrDuplicateRouter = errors.New("duplicate router path")
	ErrRouterSyntax    = errors.New("invalid router path syntax")
)

// MethodAny means any http method.
const MethodAny = "ANY"

// Params is the type for request params.
type Params map[string]string

// Router is an HTTP request multiplexer. It matches the URL of each
// incoming request against a list of registered fastrouter with their associated
// methods and calls the handler for the given URL.
type Router struct {
	trees map[string]*node
}

// node represent a router in the routers trie.
type node struct {
	router   string
	children map[string]*node
	param    string
	tag      interface{}
	star     bool
}

// NewRouter returns a new Router.
func NewRouter() *Router {
	return &Router{trees: make(map[string]*node)}
}

// Handle registers a new fastrouter with the given fastrouter and method.
func (r *Router) Handle(method string, router string, tag interface{}) error {
	// check if for given method there is not any tie create a new one.
	if _, ok := r.trees[method]; !ok {
		r.trees[method] = &node{router: "/", children: make(map[string]*node)}
	}

	return r.trees[method].add(router, tag)
}

// Search matches r.URL.Path with a stored route and calls handler for found node.
func (r *Router) Search(method, path string) (bool, Params, interface{}) {
	// check if there is a trie for the request method.
	t, ok := r.trees[method]
	if !ok && method != MethodAny { // try any
		t, ok = r.trees[MethodAny]
	}

	if !ok {
		return false, nil, nil
	}

	// find the node with request url fastrouter in the trie.
	node, params := t.find(path)
	if node != nil {
		return true, params, node.tag
	}

	// try any
	if t, ok = r.trees[MethodAny]; !ok {
		return false, nil, nil
	}

	if node, params = t.find(path); node == nil {
		return false, nil, nil
	}

	return true, params, node.tag
}

// add method adds a new router to the trie.
func (n *node) add(router string, tag interface{}) error {
	cur := n
	trimmed := strings.TrimPrefix(router, "/")
	subs := strings.Split(trimmed, "/")
	duplicate := true
	star := false

	for _, p := range subs {
		// replace keys with pattern ":abc" with "abc" for matching params.
		// replace keys with pattern "*abc" with "abc" for matching params.
		param := ""

		if len(p) > 1 && (p[0] == ':' || p[0] == '*') {
			if p[0] == '*' {
				if star {
					return ErrRouterSyntax
				}

				star = true
			}
			param = p[1:]
			p = "*"
		}

		next, ok := cur.children[p]
		if !ok {
			duplicate = false
			next = &node{
				router: router, param: param, star: star,
				children: make(map[string]*node),
			}
			cur.children[p] = next
		}
		cur = next
	}

	if duplicate {
		return ErrDuplicateRouter
	}

	cur.tag = tag
	return nil
}

// find method match the request url router with a node in trie.
func (n *node) find(path string) (*node, Params) {
	cur := n
	params := make(Params)
	slice := strings.Split(strings.TrimPrefix(path, "/"), "/")

	for i, k := range slice {
		next, ok := cur.children[k]
		if !ok {
			if next, ok = cur.children["*"]; !ok {
				// return nil if no node match the given fastrouter.
				return nil, params
			}
		}

		cur = next

		// if the node has a param add it to params map.
		if cur.param != "" {
			if cur.star {
				params[cur.param] = strings.Join(slice[i:], "/")
				return cur, params
			}

			params[cur.param] = k
		}
	}

	// return the found node and params map.
	return cur, params
}
