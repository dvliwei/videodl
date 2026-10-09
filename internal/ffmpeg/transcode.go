package ffmpeg

import (
	"context"
	"fmt"

	"videodl/internal/media"
)

type TranscodeOptions struct {
	InputPath       string
	OutputPath      string
	Profile         media.DownloadProfile
	DurationSeconds *float64
}

func (o TranscodeOptions) withDefaults() TranscodeOptions {
	if o.Profile == "" {
		o.Profile = media.ProfileOriginal
	}
	return o
}

func BuildTranscodeArgs(opts TranscodeOptions) ([]string, error) {
	opts = opts.withDefaults()

	if opts.InputPath == "" {
		return nil, fmt.Errorf("ffmpeg: transcode: empty input path")
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("ffmpeg: transcode: empty output path")
	}

	preset, ok := GetPreset(opts.Profile)
	if !ok {
		return nil, fmt.Errorf("ffmpeg: transcode: unknown profile %q", opts.Profile)
	}

	args := make([]string, 0, 24)

	args = append(args,
		"-nostdin",
		"-v", "error",
		"-y",
	)

	args = append(args,
		"-progress", "pipe:1",
		"-nostats",
	)

	args = append(args, "-i", opts.InputPath)

	args = append(args, "-map", "0:v?", "-map", "0:a?")

	args = append(args, preset.Args...)

	args = append(args, opts.OutputPath)

	return args, nil
}

func RunFFmpegTranscode(ctx context.Context, inv *Invoker, opts TranscodeOptions, sink ProgressSink) (*DownloadResult, error) {
	if inv == nil {
		return nil, fmt.Errorf("ffmpeg: RunFFmpegTranscode: nil invoker")
	}

	opts = opts.withDefaults()

	downloadOpts := DownloadOptions{
		InputURL:        opts.InputPath,
		OutputPath:      opts.OutputPath,
		Profile:         opts.Profile,
		DurationSeconds: opts.DurationSeconds,
	}

	return RunFFmpegDownload(ctx, inv, downloadOpts, sink)
}

func IsStreamCopyProfile(profile media.DownloadProfile) bool {
	p, ok := GetPreset(profile)
	if !ok {
		return true
	}
	return p.StreamCopy
}

func PresetOutputExtension(profile media.DownloadProfile) string {
	p, ok := GetPreset(profile)
	if !ok {
		return ".mp4"
	}
	return p.OutputExtension
}
