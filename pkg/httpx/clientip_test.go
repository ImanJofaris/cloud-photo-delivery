package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func clientIPReq(remote, xff string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestRemoteIP(t *testing.T) {
	if got := RemoteIP(clientIPReq("203.0.113.9:443", "")); got != "203.0.113.9" {
		t.Fatalf("got %q", got)
	}
	if got := RemoteIP(clientIPReq("[2001:db8::1]:443", "")); got != "2001:db8::1" {
		t.Fatalf("got %q", got)
	}
	if got := RemoteIP(clientIPReq("not-a-host-port", "")); got != "not-a-host-port" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPResolver_IgnoresForwardedWithoutTrustedProxy(t *testing.T) {
	c := NewClientIPResolver(nil)
	if got := c.Resolve(clientIPReq("203.0.113.9:443", "1.1.1.1")); got != "203.0.113.9" {
		t.Fatalf("spoofed X-Forwarded-For must be ignored, got %q", got)
	}
	if got := c.Resolve(clientIPReq("203.0.113.9:443", "1.1.1.1, 2.2.2.2")); got != "203.0.113.9" {
		t.Fatalf("spoofed X-Forwarded-For must be ignored, got %q", got)
	}
}

func TestClientIPResolver_TrustsForwardedFromProxy(t *testing.T) {
	c := NewClientIPResolver([]string{"127.0.0.1/32"})
	if got := c.Resolve(clientIPReq("127.0.0.1:5555", "198.51.100.7")); got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPResolver_WalksToRightmostUntrustedHop(t *testing.T) {
	c := NewClientIPResolver([]string{"127.0.0.1/32", "10.0.0.0/8"})
	if got := c.Resolve(clientIPReq("127.0.0.1:5555", "198.51.100.7, 10.0.0.5")); got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
	if got := c.Resolve(clientIPReq("10.0.0.1:5555", "198.51.100.7")); got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPResolver_AllHopsTrustedFallsBackToPeer(t *testing.T) {
	c := NewClientIPResolver([]string{"10.0.0.0/8"})
	if got := c.Resolve(clientIPReq("10.0.0.1:5555", "10.0.0.9, 10.0.0.8")); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPResolver_AcceptsBareIPEntries(t *testing.T) {
	c := NewClientIPResolver([]string{"::1"})
	if got := c.Resolve(clientIPReq("[::1]:5555", "198.51.100.7")); got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPResolver_SkipsEmptyHops(t *testing.T) {
	c := NewClientIPResolver([]string{"127.0.0.1/32"})
	if got := c.Resolve(clientIPReq("127.0.0.1:5555", " , , 198.51.100.7")); got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
	if got := c.Resolve(clientIPReq("127.0.0.1:5555", "")); got != "127.0.0.1" {
		t.Fatalf("got %q", got)
	}
}
