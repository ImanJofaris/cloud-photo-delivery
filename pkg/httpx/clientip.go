package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIPResolver resolves the client address used for rate-limit keys.
// Forwarded headers are trusted only when the direct peer is a configured
// proxy: any client can set X-Forwarded-For, so trusting it unconditionally
// lets a caller mint a fresh limiter bucket per request.
type ClientIPResolver struct {
	trusted []*net.IPNet
}

// NewClientIPResolver builds a resolver from CIDR or bare-IP entries (for
// example "127.0.0.1/32" or "10.0.0.0/8"). Invalid entries are ignored; config
// validation rejects them at startup.
func NewClientIPResolver(entries []string) *ClientIPResolver {
	r := &ClientIPResolver{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			r.trusted = append(r.trusted, network)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			continue
		}
		bits := 128
		if ip.To4() != nil {
			bits = 32
		}
		r.trusted = append(r.trusted, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return r
}

// RemoteIP returns the network address of the direct peer.
func RemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Resolve returns the client address for the request. The direct peer wins
// unless it is a trusted proxy, in which case the rightmost X-Forwarded-For
// hop that is not itself a trusted proxy is used. A chain of only trusted
// hops falls back to the peer.
func (c *ClientIPResolver) Resolve(r *http.Request) string {
	peer := RemoteIP(r)
	if len(c.trusted) == 0 || !c.trustedIP(peer) {
		return peer
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		if !c.trustedIP(hop) {
			return hop
		}
	}
	return peer
}

func (c *ClientIPResolver) trustedIP(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return false
	}
	for _, network := range c.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
