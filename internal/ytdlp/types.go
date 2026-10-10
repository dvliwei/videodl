package ytdlp

import "encoding/json"

// RunConfig bounds all output captured from the standalone executable.
type RunConfig struct {
	BinaryPath     string
	MaxStdoutBytes int64
	MaxStderrBytes int64
	Env            []string
}

// RunResult contains bounded child-process output. Callers must treat stderr
// as backend diagnostic material and must not forward it to the UI unchanged.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// BrowserSession is an explicit, per-attempt authorization choice. Cookie
// contents and browser storage paths are never represented here.
type BrowserSession struct {
	Browser string
	Profile string
}

type ExtractRequest struct {
	URL            string
	FormatSelector string
	Browser        *BrowserSession
	ProxyURL       string
}

// Info is the restricted subset of yt-dlp's single-video JSON that later
// mapping code needs. The original JSON is intentionally not retained.
type Info struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Extractor        string            `json:"extractor"`
	WebpageURL       string            `json:"webpage_url"`
	OriginalURL      string            `json:"original_url"`
	Type             string            `json:"_type"`
	Duration         *float64          `json:"duration"`
	Formats          []Format          `json:"formats"`
	RequestedFormats []Format          `json:"requested_formats"`
	Entries          []json.RawMessage `json:"entries"`
}

type Format struct {
	FormatID       string            `json:"format_id"`
	URL            string            `json:"url"`
	Ext            string            `json:"ext"`
	Protocol       string            `json:"protocol"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	FPS            float64           `json:"fps"`
	VCodec         string            `json:"vcodec"`
	ACodec         string            `json:"acodec"`
	Filesize       *int64            `json:"filesize"`
	FilesizeApprox *int64            `json:"filesize_approx"`
	TBR            float64           `json:"tbr"`
	AudioChannels  int               `json:"audio_channels"`
	HTTPHeaders    map[string]string `json:"http_headers"`
}
