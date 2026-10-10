package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *Runner) Extract(ctx context.Context, req ExtractRequest) (*Info, error) {
	if r == nil || req.URL == "" {
		return nil, fmt.Errorf("%w: URL is required", ErrInvalidRequest)
	}
	args := []string{"--dump-single-json", "--no-playlist", "--ignore-config"}
	if req.Browser == nil {
		args = append(args, "--no-cookies-from-browser")
	} else {
		if req.Browser.Browser == "" {
			return nil, fmt.Errorf("%w: browser is required", ErrInvalidRequest)
		}
		browser := req.Browser.Browser
		if req.Browser.Profile != "" {
			browser += ":" + req.Browser.Profile
		}
		args = append(args, "--cookies-from-browser", browser)
	}
	if req.ProxyURL != "" {
		args = append(args, "--proxy", req.ProxyURL)
	}
	if req.FormatSelector != "" {
		args = append(args, "--format", req.FormatSelector)
	}
	args = append(args, "--", req.URL)
	result, err := r.Run(ctx, args)
	if err != nil {
		return nil, err
	}
	var info Info
	if err := json.Unmarshal(result.Stdout, &info); err != nil {
		return nil, fmt.Errorf("yt-dlp: invalid JSON response: %w", err)
	}
	if info.Type == "playlist" || len(info.Entries) > 0 {
		return nil, fmt.Errorf("yt-dlp: playlist input is unsupported")
	}
	return &info, nil
}
