package httptee

import (
	"log"
	"net/http"
	"strings"
)

// InsertForwardedHeaders insert header X-Forwarded-For and Forwarded headers.
func InsertForwardedHeaders(request *http.Request) {
	var remoteIP string

	if pos := strings.LastIndex(request.RemoteAddr, ":"); pos != -1 {
		remoteIP = request.RemoteAddr[:pos]
	} else {
		log.Printf("The default format of req.RemoteAddr should be IP:Port but was %s\n", remoteIP)
		remoteIP = request.RemoteAddr
	}

	insertOrExtendForwardedHeader(request, remoteIP)
	insertOrExtendXFFHeader(request, remoteIP)
}

const xffHeader = "X-Forwarded-For"

func insertOrExtendXFFHeader(request *http.Request, remoteIP string) {
	header := request.Header.Get(xffHeader)

	if header != "" { // extend
		request.Header.Set(xffHeader, header+", "+remoteIP)
	} else { // insert
		request.Header.Set(xffHeader, remoteIP)
	}
}

const forwardedHeader = "Forwarded"

// Implementation according to rfc7239
func insertOrExtendForwardedHeader(request *http.Request, remoteIP string) {
	extension := "for=" + remoteIP
	header := request.Header.Get(forwardedHeader)

	if header != "" { // extend
		request.Header.Set(forwardedHeader, header+", "+extension)
	} else { // insert
		request.Header.Set(forwardedHeader, extension)
	}
}
