package removals

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

// registeredDomain returns the effective-TLD-plus-one for host, falling back to
// the trimmed host on errors (IP literals, single-label hosts like "localhost",
// empty input). The returned value is lowercased for case-insensitive comparison.
func registeredDomain(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	host = strings.TrimSuffix(host, ".")
	etld1, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}
	return etld1
}

// sameRegisteredDomain reports whether a and b share the same effective-TLD-plus-one
// (e.g. blog.example.com and www.example.com both resolve to example.com).
// Empty hostnames never match.
func sameRegisteredDomain(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return registeredDomain(a) == registeredDomain(b)
}
