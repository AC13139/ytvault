package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesSetsSecurityHeaders(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	want := map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "strict-origin-when-cross-origin",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
	for header, value := range want {
		if got := w.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestContentSecurityPolicyLocksDownOrigins(t *testing.T) {
	for _, directive := range []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"connect-src 'self'",
	} {
		if !strings.Contains(contentSecurityPolicy, directive) {
			t.Errorf("CSP missing %q; got %q", directive, contentSecurityPolicy)
		}
	}

	// The UI must not be able to load or phone out to arbitrary origins.
	for _, forbidden := range []string{"connect-src *", "frame-src *", "default-src *"} {
		if strings.Contains(contentSecurityPolicy, forbidden) {
			t.Errorf("CSP contains %q; got %q", forbidden, contentSecurityPolicy)
		}
	}
}
