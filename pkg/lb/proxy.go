package lb

import (
	"log"
	"net/http"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

// ServeHTTP ReverseProxy to serve
// ref to: https://golang.org/src/net/http/httputil/reverseproxy.go#L169
func ServeHTTP(w http.ResponseWriter, r *http.Request, h HandlerFunc) error {
	headerSaver := saveHeaders(w.Header())
	headerHop := MakeHeaderHop()

	headerHop.Del(r.Header)

	log.Printf("recv a requests to proxy to: %s", r.RemoteAddr)

	if err := h(w, r); err != nil {
		log.Printf("could not proxy: %v\n", err)
		return err
	}

	// response to client
	headerHop.Del(w.Header())
	headerSaver.set(&r.Header)

	return nil
}
