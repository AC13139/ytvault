package api

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

// basicAuth guards the API with HTTP Basic authentication.
//
// It is opt-in: when no credentials are configured the middleware passes
// every request through, so existing deployments are unaffected. When
// credentials are set, every route except /healthz requires them.
//
// Basic auth was chosen over a token in a custom header because the
// browser handles it natively — it prompts once and then attaches the
// credentials to every request, so the web UI needs no changes at all.
type basicAuth struct {
	user     string
	password string
}

// newBasicAuth returns a guard that is disabled unless both a user and a
// password are supplied. A half-configured guard would either lock
// everyone out or authenticate nobody, so it stays disabled instead.
func newBasicAuth(user, password string) *basicAuth {
	return &basicAuth{user: user, password: password}
}

func (a *basicAuth) enabled() bool {
	return a.user != "" && a.password != ""
}

// matches compares the supplied credentials in constant time.
func (a *basicAuth) matches(user, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.user)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
	return userOK && passOK
}

// middleware enforces authentication and rejects cross-origin requests.
func (a *basicAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.enabled() {
			next.ServeHTTP(w, r)
			return
		}

		// The Docker healthcheck has no credentials, so the liveness probe
		// stays reachable.
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		if !a.isSameOrigin(r) {
			writeError(w, http.StatusForbidden, "forbidden",
				"cross-origin requests are not allowed")
			return
		}

		user, password, ok := r.BasicAuth()
		if !ok || !a.matches(user, password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="YtVault", charset="UTF-8"`)
			writeError(w, http.StatusUnauthorized, "unauthorized",
				"authentication required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isSameOrigin rejects state-changing requests that a web page on another
// origin triggered.
//
// The browser attaches cached Basic credentials to any request to the
// host, so without this check a page on another site could drive the API
// on the user's behalf. Browsers send Origin on every non-GET request and
// Sec-Fetch-Site on every request; either one is enough to tell that the
// request came from elsewhere. Command-line clients send neither, and are
// not subject to this attack.
func (a *basicAuth) isSameOrigin(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}

	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
