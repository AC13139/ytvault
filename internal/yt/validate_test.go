package yt

import (
	"strings"
	"testing"

	"ytvault/internal/models"
)

func TestValidateSource(t *testing.T) {
	cases := []struct {
		name    string
		kind    models.SourceKind
		url     string
		wantErr bool
	}{
		{"video watch ok", models.SourceKindVideo, "https://www.youtube.com/watch?v=abc123", false},
		{"video short ok", models.SourceKindVideo, "https://youtu.be/abc123", false},
		{"playlist ok", models.SourceKindPlaylist, "https://www.youtube.com/playlist?list=PL123", false},
		{"channel handle ok", models.SourceKindChannel, "https://www.youtube.com/@somebody", false},
		{"channel bad", models.SourceKindChannel, "https://www.youtube.com/watch?v=abc", true},
		{"playlist bad", models.SourceKindPlaylist, "https://www.youtube.com/watch?v=abc", true},
		{"non-youtube bad", models.SourceKindVideo, "https://vimeo.com/1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSource(tc.url, tc.kind)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSource() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeSource_ChannelHandleHTTP(t *testing.T) {
	profile, err := NormalizeSource("http://www.youtube.com/@SpeechLab-m7o", models.SourceKindChannel)
	if err != nil {
		t.Fatalf("NormalizeSource() error = %v", err)
	}
	if profile.NormalizedURL != "https://www.youtube.com/@SpeechLab-m7o" {
		t.Fatalf("normalized url = %q", profile.NormalizedURL)
	}
	if profile.Locator != "@SpeechLab-m7o" {
		t.Fatalf("locator = %q", profile.Locator)
	}
	if profile.DisplayName != "Channel @SpeechLab-m7o" {
		t.Fatalf("display name = %q", profile.DisplayName)
	}
	if profile.PathKind != "handle" || profile.PathLabel != "@handle" {
		t.Fatalf("path = %q / %q", profile.PathKind, profile.PathLabel)
	}
	if profile.IntakeStage != "validated" {
		t.Fatalf("intake stage = %q", profile.IntakeStage)
	}
	if !strings.Contains(profile.NextAction, "index task stub") {
		t.Fatalf("next action = %q", profile.NextAction)
	}
}

func TestNormalizeSource_ChannelVariants(t *testing.T) {
	cases := []struct {
		name          string
		url           string
		wantURL       string
		wantPathKind  string
		wantPathLabel string
	}{
		{"mobile handle", "https://m.youtube.com/@somebody", "https://www.youtube.com/@somebody", "handle", "@handle"},
		{"channel id", "https://www.youtube.com/channel/UC123", "https://www.youtube.com/channel/UC123", "channel-id", "canonical channel id"},
		{"custom path", "https://www.youtube.com/c/demo", "https://www.youtube.com/c/demo", "custom-path", "legacy /c path"},
		{"user path", "https://www.youtube.com/user/demo", "https://www.youtube.com/user/demo", "user-path", "legacy /user path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile, err := NormalizeSource(tc.url, models.SourceKindChannel)
			if err != nil {
				t.Fatalf("NormalizeSource() error = %v", err)
			}
			if profile.NormalizedURL != tc.wantURL {
				t.Fatalf("normalized url = %q, want %q", profile.NormalizedURL, tc.wantURL)
			}
			if profile.PathKind != tc.wantPathKind || profile.PathLabel != tc.wantPathLabel {
				t.Fatalf("path = %q / %q, want %q / %q", profile.PathKind, profile.PathLabel, tc.wantPathKind, tc.wantPathLabel)
			}
		})
	}
}
