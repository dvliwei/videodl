package ytdlp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"videodl/internal/media"
)

func TestMapInfoToCandidate_CombinedFormat(t *testing.T) {
	info := loadInfo(t, "single.json")
	candidate, err := MapInfoToCandidate(info)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.SourceType != media.SourceYTDLP || candidate.Title != "Example video" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.DisplayURL != "https://video.example/watch" {
		t.Fatalf("display URL = %q", candidate.DisplayURL)
	}
	if candidate.DurationSeconds == nil || *candidate.DurationSeconds != 12.5 {
		t.Fatalf("duration = %#v", candidate.DurationSeconds)
	}
	if candidate.SizeBytes == nil || *candidate.SizeBytes != 123456 {
		t.Fatalf("size = %#v", candidate.SizeBytes)
	}
	if !candidate.HasVideo || !candidate.HasAudio || len(candidate.Variants) != 1 {
		t.Fatalf("media flags/variants = %#v", candidate)
	}
	variant := candidate.Variants[0]
	if !variant.HasVideo || !variant.HasAudio || variant.InternalFormatSelector != "18" {
		t.Fatalf("variant = %#v", variant)
	}
	if candidate.InternalYTDLPSource == nil || candidate.InternalYTDLPSource.PageURL == "" {
		t.Fatalf("missing backend source: %#v", candidate.InternalYTDLPSource)
	}
}

func TestMapInfoToCandidate_SeparateVideoAndAudio(t *testing.T) {
	info := loadInfo(t, "separate-av.json")
	first, err := MapInfoToCandidate(info)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MapInfoToCandidate(info)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(first.Variants) != 2 || first.HasVideo != true || first.HasAudio != true {
		t.Fatalf("stable mapping = %#v / %#v", first, second)
	}
	if first.Variants[0].ID != second.Variants[0].ID || first.Variants[1].ID != second.Variants[1].ID {
		t.Fatalf("variant IDs are not stable: %#v / %#v", first.Variants, second.Variants)
	}
	if first.Variants[0].HasAudio || !first.Variants[0].HasVideo || first.Variants[1].HasVideo || !first.Variants[1].HasAudio {
		t.Fatalf("stream flags = %#v", first.Variants)
	}
	if first.InternalYTDLPSource == nil || first.InternalYTDLPSource.FormatSelector != "137+140" {
		t.Fatalf("default selector = %#v", first.InternalYTDLPSource)
	}
}

func TestMapInfoToCandidate_MissingOptionalDataAndUnknownCodec(t *testing.T) {
	info := &Info{
		ID:    "unknown-codec",
		Title: "Unknown codec",
		Formats: []Format{{
			FormatID: "x",
			URL:      "https://cdn.example/stream",
			VCodec:   "unknown",
			ACodec:   "unknown",
		}},
	}
	candidate, err := MapInfoToCandidate(info)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.DurationSeconds != nil || candidate.SizeBytes != nil {
		t.Fatalf("optional values should be nil: %#v", candidate)
	}
	if candidate.HasVideo || candidate.HasAudio {
		t.Fatalf("unknown codecs should not be claimed as playable: %#v", candidate)
	}
}

func TestMapInfoToCandidate_RejectsPlaylistAndNoFormats(t *testing.T) {
	for name, info := range map[string]*Info{
		"playlist":   {Type: "playlist", ID: "p", Formats: []Format{{FormatID: "1", URL: "https://cdn.example/a"}}},
		"no formats": {ID: "empty", Title: "empty"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MapInfoToCandidate(info)
			if err == nil {
				t.Fatal("expected mapping error")
			}
			if name == "playlist" && !errors.Is(err, ErrPlaylistInput) {
				t.Fatalf("expected playlist error, got %v", err)
			}
			if name == "no formats" && !errors.Is(err, ErrNoFormats) {
				t.Fatalf("expected no formats error, got %v", err)
			}
		})
	}
}

func TestValidateBrowserSession_AllowlistAndProfileSafety(t *testing.T) {
	for _, browser := range []string{"brave", "chrome", "chromium", "edge", "firefox", "opera", "safari", "vivaldi", "whale"} {
		if err := ValidateBrowserSession(BrowserSession{Browser: browser, Profile: "Profile 1"}); err != nil {
			t.Errorf("browser %q rejected: %v", browser, err)
		}
	}
	for _, session := range []BrowserSession{
		{Browser: "unknown"},
		{Browser: "chrome", Profile: "../../secrets"},
		{Browser: "chrome", Profile: `C:\Users\me`},
		{Browser: "chrome", Profile: "--exec=touch"},
		{Browser: "chrome", Profile: "Profile\n1"},
	} {
		if err := ValidateBrowserSession(session); !errors.Is(err, ErrInvalidBrowser) {
			t.Errorf("expected invalid browser for %#v, got %v", session, err)
		}
	}
}

func TestMapInfoToCandidate_JSONDoesNotExposeAuthority(t *testing.T) {
	candidate, err := MapInfoToCandidate(loadInfo(t, "single.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, forbidden := range []string{"token=secret", "http_headers", "Referer", "format_id", "InternalYTDLPSource", "bestvideo"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("forbidden value %q exposed in %s", forbidden, serialized)
		}
	}
}

func loadInfo(t *testing.T, name string) *Info {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	return &info
}
