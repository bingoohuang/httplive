package acl

import (
	"strings"
)

// each node represent a path in the router trie.
type node struct {
	path     string
	children map[string]*node
	param    string
	tag      interface{}
	star     bool
}

// add method adds a new path to the trie.
func (n *node) add(path string, tag interface{}) {
	cur := n
	trimmed := strings.TrimPrefix(path, "/")
	subs := strings.Split(trimmed, "/")

	for _, p := range subs {
		// replace keys with pattern ":*" with "*" for matching params.
		param := ""

		star := false
		if len(p) > 1 && (p[0] == ':' || p[0] == '*') {
			star = p[0] == '*'
			param = p[1:]
			p = "*"
		}

		next, ok := cur.children[p]
		if !ok {
			next = &node{
				path:     path,
				children: make(map[string]*node),
				param:    param,
				star:     star,
			}
			cur.children[p] = next
		}
		cur = next
	}

	cur.tag = tag
}

// find method match the request url path with a node in trie.
func (n *node) find(path string) (*node, Params) {
	params := make(Params)
	cur := n
	trimmed := strings.TrimPrefix(path, "/")
	slice := strings.Split(trimmed, "/")

	for i, k := range slice {
		next, ok := cur.children[k]
		if !ok {
			if next, ok = cur.children["*"]; !ok {
				// return nil if no node match the given path.
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

// Params is the type for request params.
type Params map[string]string

// Router is an HTTP request multiplexer. It matches the URL of each
// incoming request against a list of registered path with their associated
// methods and calls the handler for the given URL.
type Router struct {
	trees map[string]*node
}

// NewRouter returns a new Router.
func NewRouter() *Router {
	return &Router{
		trees: make(map[string]*node),
	}
}

// ServeHTTP matches r.URL.Path with a stored route and calls handler for found node.
func (r *Router) Search(method, path string) (bool, Params, interface{}) {
	// check if there is a trie for the request method.
	t, ok := r.trees[method]
	if !ok {
		return false, nil, nil
	}

	// find the node with request url path in the trie.
	node, params := t.find(path)
	if node == nil {
		return false, nil, nil
	}

	return true, params, node.tag
}

// Handle registers a new path with the given path and method.
func (r *Router) Handle(method string, path string, tag interface{}) {
	// check if for given method there is not any tie create a new one.
	if _, ok := r.trees[method]; !ok {
		r.trees[method] = &node{
			path:     "/",
			children: make(map[string]*node),
		}
	}

	r.trees[method].add(path, tag)
}
