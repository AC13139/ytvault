package api

import "net/http"

// contentSecurityPolicy restricts where the browser is allowed to load
// resources from.
//
// The web UI ships a single inline script plus inline event handlers, so
// script-src needs 'unsafe-inline'. Everything else is locked down: no
// framing, no plugins, no outbound XHR/fetch to other origins, and images
// only from YouTube's CDN.
const contentSecurityPolicy = "default-src 'self'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"object-src 'none'; " +
	"script-src 'self' 'unsafe-inline'; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data: https://img.youtube.com https://*.ytimg.com " +
	"https://*.ggpht.com https://*.googleusercontent.com; " +
	"connect-src 'self'"

// withSecurityHeaders adds defensive response headers to every response.
//
// YtVault has no built-in authentication, so any client that can reach the
// port can drive the API. These headers do not authenticate anything — they
// limit the blast radius of content injected into the server-rendered UI:
// clickjacking (frame-ancestors / X-Frame-Options), MIME sniffing, and
// script injection that phones home to an attacker-controlled origin
// (connect-src 'self').
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
