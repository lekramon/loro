package downloadclient

import (
	"net/url"
	"testing"
)

func TestReverseProxyEscapesPreserveEndpointIdentity(t *testing.T) {
	base, err := url.Parse("https://localhost:8081/qbit%2Fproxy")
	if err != nil {
		t.Fatal(err)
	}
	s := Session{base: base}
	if request := s.endpoint("auth/login").String(); request != "https://localhost:8081/qbit%2Fproxy/api/v2/auth/login" {
		t.Fatal("encoded reverse-proxy path changed", request)
	}
}
