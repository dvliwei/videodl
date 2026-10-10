package ffmpeg

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"videodl/internal/proxy"

	"videodl/internal/media"
)

type Input struct {
	URL     string
	Headers map[string]string
}

type DownloadOptions struct {
	Inputs []Input
	// InputURL is retained for native and transcode callers during the
	// migration to Inputs. New code should use Inputs.
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

	inputs, err := normalizeInputs(opts)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("ffmpeg: download: empty input URL")
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("ffmpeg: download: empty output path")
	}

	args := make([]string, 0, 32+len(inputs)*6)

	args = append(args,
		"-nostdin",
		"-v", "error",
		"-y",
	)

	args = append(args,
		"-protocol_whitelist", protocolWhitelistForInputs(inputs),
	)

	if proxyURL := proxy.ResolveSystemProxyURL(); proxyURL != "" {
		args = append(args, "-http_proxy", proxyURL)
	}

	args = append(args,
		"-progress", "pipe:1",
		"-nostats",
	)

	for _, input := range inputs {
		headerValue := inputHeaderValue(opts, input)
		if headerValue != "" {
			args = append(args, "-headers", headerValue)
		}
		args = append(args, "-i", input.URL)
	}

	if len(inputs) > 1 {
		for i := range inputs {
			args = append(args, "-map", fmt.Sprintf("%d:v?", i), "-map", fmt.Sprintf("%d:a?", i))
		}
	} else {
		args = append(args, selectStreamArgs(opts)...)
	}

	preset, ok := GetPreset(opts.Profile)
	if !ok {
		return nil, fmt.Errorf("ffmpeg: download: unknown profile %q", opts.Profile)
	}
	args = append(args, preset.Args...)

	if preset.StreamCopy {
		args = append(args, containerFlags(opts.OutputPath)...)
	}

	args = append(args, opts.OutputPath)

	return args, nil
}

func normalizeInputs(opts DownloadOptions) ([]Input, error) {
	if len(opts.Inputs) == 0 {
		if opts.InputURL == "" {
			return nil, fmt.Errorf("ffmpeg: download: empty input URL")
		}
		return []Input{{URL: opts.InputURL}}, nil
	}
	inputs := make([]Input, len(opts.Inputs))
	copy(inputs, opts.Inputs)
	for i := range inputs {
		if inputs[i].URL == "" {
			return nil, fmt.Errorf("ffmpeg: download: input %d has empty URL", i)
		}
		u, err := url.Parse(inputs[i].URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("ffmpeg: download: input %d must be an HTTP or HTTPS URL", i)
		}
	}
	return inputs, nil
}

func inputHeaderValue(opts DownloadOptions, input Input) string {
	headers := make(map[string]string, len(opts.Headers)+len(input.Headers)+2)
	keys := make(map[string]string, len(headers))
	set := func(key, value string) {
		if key == "" {
			return
		}
		lower := strings.ToLower(key)
		if existing, ok := keys[lower]; ok {
			delete(headers, existing)
		}
		headers[key] = value
		keys[lower] = key
	}
	if opts.Referer != "" {
		set("Referer", opts.Referer)
	}
	if opts.UserAgent != "" {
		set("User-Agent", opts.UserAgent)
	}
	for key, value := range opts.Headers {
		set(key, value)
	}
	for key, value := range input.Headers {
		set(key, value)
	}
	ordered := make([]string, 0, len(headers))
	for key := range headers {
		ordered = append(ordered, key)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return strings.ToLower(ordered[i]) < strings.ToLower(ordered[j]) })
	parts := make([]string, 0, len(ordered))
	for _, key := range ordered {
		parts = append(parts, key+": "+headers[key])
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\r\n") + "\r\n"
}

func protocolWhitelistForInputs(inputs []Input) string {
	for _, input := range inputs {
		if strings.Contains(protocolWhitelistFor(input.URL), "file") {
			return allProtocols
		}
	}
	return networkProtocols
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

const (
	networkProtocols = "http,https,tcp,tls,crypto,httpproxy"
	allProtocols     = "file," + networkProtocols
)

func protocolWhitelistFor(inputURL string) string {
	u, err := url.Parse(inputURL)
	if err != nil || u.Scheme == "" {
		return allProtocols
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return networkProtocols
	}
	return networkProtocols
}
