package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeRejectsNonYouTubeURL(t *testing.T) {
	app := newTestApp(t)

	cases := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:8080/api/v1/sources",
		"https://vimeo.com/1",
		// A leading dash is read by yt-dlp as an option, not a URL.
		"--config-location=/etc/passwd",
		"--plugin-dirs=/tmp/evil",
	}
	for _, raw := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/youtube/probe?url="+raw, nil)
		w := httptest.NewRecorder()
		app.Routes().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("probe(%q) status = %d, want 400", raw, w.Code)
		}
	}
}

func TestPosterFetchRejectsInternalAddress(t *testing.T) {
	app := newTestApp(t)

	body := []byte(`{"kind":"video","url":"https://www.youtube.com/watch?v=abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(body))
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create source status = %d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Source struct {
			ID string `json:"id"`
		} `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Source.ID == "" {
		t.Fatal("source id missing")
	}

	for _, raw := range []string{
		"https://127.0.0.1/poster.jpg",
		"https://169.254.169.254/latest/meta-data/",
		"https://[::]/poster.jpg",
	} {
		posterBody := []byte(`{"url":"` + raw + `"}`)
		posterReq := httptest.NewRequest(http.MethodPost,
			"/api/v1/sources/"+created.Source.ID+"/poster", bytes.NewReader(posterBody))
		posterW := httptest.NewRecorder()
		app.Routes().ServeHTTP(posterW, posterReq)

		if posterW.Code != http.StatusBadRequest {
			t.Errorf("poster(%q) status = %d, want 400", raw, posterW.Code)
		}
	}
}
