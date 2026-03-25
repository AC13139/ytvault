package yt

import (
	"fmt"
	"net/url"
	"strings"

	"ytvault/internal/models"
)

type SourceProfile struct {
	InputURL           string `json:"inputUrl"`
	NormalizedURL      string `json:"normalizedUrl"`
	Host               string `json:"host"`
	Locator            string `json:"locator"`
	DisplayName        string `json:"displayName"`
	PathKind           string `json:"pathKind"`
	PathLabel          string `json:"pathLabel"`
	IntakeStage        string `json:"intakeStage"`
	NextAction         string `json:"nextAction"`
	ValidationSummary  string `json:"validationSummary"`
	PrototypeReadiness string `json:"prototypeReadiness"`
}

func ValidateSource(raw string, kind models.SourceKind) error {
	_, err := NormalizeSource(raw, kind)
	return err
}

func NormalizeSource(raw string, kind models.SourceKind) (SourceProfile, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return SourceProfile{}, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return SourceProfile{}, fmt.Errorf("url must start with http or https")
	}
	host := strings.ToLower(u.Hostname())
	if host != "youtube.com" && host != "www.youtube.com" && host != "youtu.be" && host != "m.youtube.com" {
		return SourceProfile{}, fmt.Errorf("only YouTube URLs are supported in phase 1")
	}

	baseHost := "www.youtube.com"
	if host == "youtu.be" {
		baseHost = "youtu.be"
	}
	path := strings.Trim(u.Path, "/")
	profile := SourceProfile{
		InputURL:           raw,
		Host:               host,
		IntakeStage:        "validated",
		NextAction:         "Source is stored and an index task stub is queued. Live indexing/download still needs a worker phase.",
		PrototypeReadiness: "validated-only: source accepted and queued, live indexing/download not wired yet",
	}

	switch kind {
	case models.SourceKindVideo:
		if host == "youtu.be" {
			if path == "" {
				return SourceProfile{}, fmt.Errorf("video url must include a video id after youtu.be/")
			}
			profile.Locator = path
			profile.NormalizedURL = "https://youtu.be/" + path
			profile.DisplayName = "Video (pending index)"
			profile.PathKind = "short-link"
			profile.PathLabel = "youtu.be short link"
			profile.ValidationSummary = "accepted as single-video source via youtu.be short link"
			return profile, nil
		}
		videoID := strings.TrimSpace(u.Query().Get("v"))
		if videoID == "" {
			return SourceProfile{}, fmt.Errorf("video url must include v=<id> or use youtu.be/<id>")
		}
		profile.Locator = videoID
		profile.NormalizedURL = "https://" + baseHost + "/watch?v=" + videoID
		profile.DisplayName = "Video (pending index)"
		profile.PathKind = "watch"
		profile.PathLabel = "watch page"
		profile.ValidationSummary = "accepted as single-video source via watch page"
	case models.SourceKindPlaylist:
		playlistID := strings.TrimSpace(u.Query().Get("list"))
		if playlistID == "" {
			return SourceProfile{}, fmt.Errorf("playlist url must include list=<id>")
		}
		profile.Locator = playlistID
		profile.NormalizedURL = "https://" + baseHost + "/playlist?list=" + playlistID
		profile.DisplayName = "Playlist (pending index)"
		profile.PathKind = "playlist"
		profile.PathLabel = "playlist page"
		profile.ValidationSummary = "accepted as recurring playlist source"
	case models.SourceKindChannel:
		if path == "" {
			return SourceProfile{}, fmt.Errorf("channel url must look like /channel/<id>, /@handle, /c/<name>, or /user/<name>")
		}
		switch {
		case strings.HasPrefix(path, "@"):
			profile.Locator = path
			profile.NormalizedURL = "https://www.youtube.com/" + path
			profile.DisplayName = "Channel (pending index)"
			profile.PathKind = "handle"
			profile.PathLabel = "@handle"
			profile.ValidationSummary = "accepted as channel source by handle"
		case strings.HasPrefix(path, "channel/"):
			id := strings.TrimPrefix(path, "channel/")
			if id == "" {
				return SourceProfile{}, fmt.Errorf("channel url must include a channel id after /channel/")
			}
			profile.Locator = id
			profile.NormalizedURL = "https://www.youtube.com/channel/" + id
			profile.DisplayName = "Channel (pending index)"
			profile.PathKind = "channel-id"
			profile.PathLabel = "canonical channel id"
			profile.ValidationSummary = "accepted as channel source by canonical channel id"
		case strings.HasPrefix(path, "c/"):
			name := strings.TrimPrefix(path, "c/")
			if name == "" {
				return SourceProfile{}, fmt.Errorf("channel url must include a custom name after /c/")
			}
			profile.Locator = "c/" + name
			profile.NormalizedURL = "https://www.youtube.com/c/" + name
			profile.DisplayName = "Channel (pending index)"
			profile.PathKind = "custom-path"
			profile.PathLabel = "legacy /c path"
			profile.ValidationSummary = "accepted as channel source by legacy custom path"
		case strings.HasPrefix(path, "user/"):
			name := strings.TrimPrefix(path, "user/")
			if name == "" {
				return SourceProfile{}, fmt.Errorf("channel url must include a user name after /user/")
			}
			profile.Locator = "user/" + name
			profile.NormalizedURL = "https://www.youtube.com/user/" + name
			profile.DisplayName = "Channel (pending index)"
			profile.PathKind = "user-path"
			profile.PathLabel = "legacy /user path"
			profile.ValidationSummary = "accepted as channel source by legacy user path"
		default:
			return SourceProfile{}, fmt.Errorf("channel url must look like /channel/<id>, /@handle, /c/<name>, or /user/<name>")
		}
	default:
		return SourceProfile{}, fmt.Errorf("unsupported source kind")
	}

	return profile, nil
}
