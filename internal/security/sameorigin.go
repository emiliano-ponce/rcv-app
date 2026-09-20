package security

import (
	"net/http"
	"net/url"
	"strings"
)

// IsSameOrigin reports whether the request's Origin (or, failing that,
// Referer) header matches the host the request was sent to. Browsers always
// send one of these headers for cross-origin fetch/XHR requests, so a
// mismatch (or absence of both) indicates a request that did not originate
// from this app's own pages.
func IsSameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if origin == "" {
		return false
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return u.Host == r.Host
}

// WrapRequireSameOrigin rejects requests whose Origin/Referer host does not
// match the request host, guarding state-changing endpoints against CSRF.
func WrapRequireSameOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsSameOrigin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
