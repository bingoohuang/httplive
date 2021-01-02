package httptee

import (
	"net/http"
	"testing"
)

const remoteAddr = "192.168.0.1:80"

func TestNoHeaderProvided(t *testing.T) {
	adserverRequest, _ := http.NewRequest("GET", "ad1/test", nil)
	adserverRequest.RemoteAddr = remoteAddr
	InsertForwardedHeaders(adserverRequest)

	xffHeader := adserverRequest.Header.Get("X-FORWARDED-FOR")
	forwardedHeader := adserverRequest.Header.Get("FORWARDED")

	if expectation := "192.168.0.1"; xffHeader != expectation {
		t.Errorf("Expected ''%s'', but received ''%s''", expectation, xffHeader)
	}

	if expectation := "for=192.168.0.1"; forwardedHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, forwardedHeader)
	}
}

func TestOnlyXFFProvided(t *testing.T) {
	adserverRequest, _ := http.NewRequest("GET", "ad1/test", nil)
	adserverRequest.RemoteAddr = remoteAddr
	adserverRequest.Header.Add("X-FORWARDED-FOR", "172.20.2.5")
	InsertForwardedHeaders(adserverRequest)

	xffHeader := adserverRequest.Header.Get("X-FORWARDED-FOR")
	forwardedHeader := adserverRequest.Header.Get("FORWARDED")

	if expectation := "172.20.2.5, 192.168.0.1"; xffHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, xffHeader)
	}

	if expectation := "for=192.168.0.1"; forwardedHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, forwardedHeader)
	}
}

func TestOnlyForwardedProvided(t *testing.T) {
	adserverRequest, _ := http.NewRequest("GET", "ad1/test", nil)
	adserverRequest.RemoteAddr = remoteAddr
	adserverRequest.Header.Add("FORWARDED", "for=172.20.2.5")
	InsertForwardedHeaders(adserverRequest)

	xffHeader := adserverRequest.Header.Get("X-FORWARDED-FOR")
	forwardedHeader := adserverRequest.Header.Get("FORWARDED")

	if expectation := "192.168.0.1"; xffHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, xffHeader)
	}

	if expectation := "for=172.20.2.5, for=192.168.0.1"; forwardedHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, forwardedHeader)
	}
}

func TestBothProvided(t *testing.T) {
	adserverRequest, _ := http.NewRequest("GET", "ad1/test", nil)
	adserverRequest.RemoteAddr = remoteAddr
	adserverRequest.Header.Add("FORWARDED", "for=172.20.2.5")
	adserverRequest.Header.Add("X-FORWARDED-FOR", "172.20.2.5")
	InsertForwardedHeaders(adserverRequest)

	xffHeader := adserverRequest.Header.Get("X-FORWARDED-FOR")
	forwardedHeader := adserverRequest.Header.Get("FORWARDED")

	if expectation := "172.20.2.5, 192.168.0.1"; xffHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, xffHeader)
	}

	if expectation := "for=172.20.2.5, for=192.168.0.1"; forwardedHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, forwardedHeader)
	}
}

func TestBothProvidedWithMoreProxies(t *testing.T) {
	adserverRequest, _ := http.NewRequest("GET", "ad1/test", nil)
	adserverRequest.RemoteAddr = "192.168.0.15:80"
	adserverRequest.Header.Add("FORWARDED", "for=172.20.2.5, for=172.20.2.36")
	adserverRequest.Header.Add("X-FORWARDED-FOR", "172.20.2.5, 172.20.2.36")
	InsertForwardedHeaders(adserverRequest)

	xffHeader := adserverRequest.Header.Get("X-FORWARDED-FOR")
	forwardedHeader := adserverRequest.Header.Get("FORWARDED")

	if expectation := "172.20.2.5, 172.20.2.36, 192.168.0.15"; xffHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, xffHeader)
	}

	if expectation := "for=172.20.2.5, for=172.20.2.36, for=192.168.0.15"; forwardedHeader != expectation {
		t.Errorf("Expected '%s', but received '%s'", expectation, forwardedHeader)
	}
}
