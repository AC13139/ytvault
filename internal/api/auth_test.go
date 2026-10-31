package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func newAuthedTestApp(t *testing.T, user, password string) *App {
	t.Helper()
	app, err := NewApp(Config{
		DataPath:     filepath.Join(t.TempDir(), "state.json"),
		DownloadDir:  t.TempDir(),
		CookiesPath:  filepath.Join(t.TempDir(), "cookies.txt"),
		Version:      "test",
		Commit:       "test",
		AuthUser:     user,
		AuthPassword: password,
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestNoCredentialsLeavesAPIOpen(t *testing.T) {
	app := newAuthedTestApp(t, "", "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (auth disabled)", w.Code)
	}
}

func TestAuthRequiresCredentials(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("WWW-Authenticate header missing")
	}
}

func TestAuthAcceptsCorrectCredentials(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.SetBasicAuth("admin", "s3cret")
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestAuthRejectsWrongPassword(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHealthzStaysUnauthenticated(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (healthcheck has no credentials)", w.Code)
	}
}

func TestHalfConfiguredCredentialsLeaveAPIOpen(t *testing.T) {
	// A user with no password would lock everyone out, so the guard stays
	// disabled rather than half-enabling.
	app := newAuthedTestApp(t, "admin", "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (auth disabled)", w.Code)
	}
}

func TestCrossOriginMutationIsRejected(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", nil)
	req.SetBasicAuth("admin", "s3cret")
	req.Header.Set("Origin", "https://attacker.example")
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestSameOriginMutationIsAllowed(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", nil)
	req.SetBasicAuth("admin", "s3cret")
	req.Header.Set("Origin", "http://ytvault.local")
	req.Host = "ytvault.local"
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("status = 403, want the request to be allowed")
	}
}

func TestCrossSiteFetchHeaderIsRejected(t *testing.T) {
	app := newAuthedTestApp(t, "admin", "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", nil)
	req.SetBasicAuth("admin", "s3cret")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}
