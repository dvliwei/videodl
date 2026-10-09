package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strings"

	"videodl/internal/media"
)

type DownloadOptions struct {
	InputURL        string
	SourceType      media.SourceType
	OutputPath      string
	Profile         media.DownloadProfile
	Referer         string
	UserAgent       string
	Headers         map[string]string
	DurationSeconds *float64
}

func (o DownloadOptions) withDefaults() DownloadOptions {
	if o.Profile == "" {
		o.Profile = media.ProfileOriginal
	}
	return o
}

func BuildDownloadArgs(opts DownloadOptions) ([]string, error) {
	opts = opts.withDefaults()

	if opts.InputURL == "" {
		return nil, fmt.Errorf("ffmpeg: download: empty input URL")
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("ffmpeg: download: empty output path")
	}

	args := make([]string, 0, 32)

	args = append(args,
		"-nostdin",
		"-v", "error",
		"-y",
	)

	if opts.Referer != "" || opts.UserAgent != "" || len(opts.Headers) > 0 {
		headerParts := make([]string, 0, len(opts.Headers)+2)
		if opts.Referer != "" {
			headerParts = append(headerParts, "Referer: "+opts.Referer)
		}
		if opts.UserAgent != "" {
			headerParts = append(headerParts, "User-Agent: "+opts.UserAgent)
		}
		for k, v := range opts.Headers {
			if strings.EqualFold(k, "Referer") || strings.EqualFold(k, "User-Agent") {
				continue
			}
			headerParts = append(headerParts, k+": "+v)
		}
		if len(headerParts) > 0 {
			args = append(args, "-headers", strings.Join(headerParts, "\r\n")+"\r\n")
		}
	}

	args = append(args,
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
	)

	args = append(args,
		"-progress", "pipe:1",
		"-nostats",
	)

	args = append(args, "-i", opts.InputURL)

	args = append(args, selectStreamArgs(opts)...)

	switch opts.Profile {
	case media.ProfileMP4:
		args = append(args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-crf", "23",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
		)
	default:
		args = append(args, "-c", "copy")
		args = append(args, containerFlags(opts.OutputPath)...)
	}

	args = append(args, opts.OutputPath)

	return args, nil
}

func selectStreamArgs(opts DownloadOptions) []string {
	if opts.SourceType == media.SourceDASH {
		return []string{
			"-map", "0:v?",
			"-map", "0:a?",
		}
	}
	return nil
}

func containerFlags(outputPath string) []string {
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".mp4", ".m4v", ".m4a":
		return []string{"-movflags", "+faststart"}
	}
	return nil
}
