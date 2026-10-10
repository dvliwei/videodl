// Package media contains data shared by the analyzer, download manager, and Wails boundary.
// All JSON field names are the single source of truth; the frontend must not define
// duplicate contracts that drift from these tags.
package media

import "time"

// SourceType identifies a media input format understood by the download pipeline.
type SourceType string

const (
	SourceDirect SourceType = "direct"
	SourceHLS    SourceType = "hls"
	SourceDASH   SourceType = "dash"
	SourceYTDLP  SourceType = "yt-dlp"
)

// DownloadProfile selects stream-copy output or an MVP compatibility transcode.
type DownloadProfile string

const (
	ProfileOriginal DownloadProfile = "original"
	ProfileMP4      DownloadProfile = "mp4-h264-aac"
)

// AnalysisResult is the result of analyzing one page. IDs are opaque references
// used by the backend; the frontend should not construct media URLs for downloads.
type AnalysisResult struct {
	ID         string           `json:"id"`
	PageTitle  string           `json:"pageTitle"`
	Candidates []MediaCandidate `json:"candidates"`
	Warnings   []string         `json:"warnings,omitempty"`
}

// MediaCandidate describes a downloadable media item without exposing a URL as
// an authority-bearing download parameter. InternalSourceURL and
// InternalVariantManifests carry the real download addresses for the backend
// pipeline only and are never serialized to the frontend.
type MediaCandidate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// DisplayURL must omit query and fragment values that may contain credentials.
	DisplayURL      string         `json:"displayUrl,omitempty"`
	SourceType      SourceType     `json:"sourceType"`
	Format          string         `json:"format,omitempty"`
	Width           int            `json:"width,omitempty"`
	Height          int            `json:"height,omitempty"`
	DurationSeconds *float64       `json:"durationSeconds,omitempty"`
	SizeBytes       *int64         `json:"sizeBytes,omitempty"`
	HasVideo        bool           `json:"hasVideo"`
	HasAudio        bool           `json:"hasAudio"`
	Variants        []MediaVariant `json:"variants,omitempty"`
	Unsupported     string         `json:"unsupported,omitempty"`

	// Internal fields — not serialized to the frontend.

	// InternalSourceURL is the real URL FFmpeg will open. For direct sources
	// it is the absolute media URL. For HLS it is the master manifest URL.
	// For DASH it is the MPD URL.
	InternalSourceURL string `json:"-"`

	// InternalVariantManifests maps a VariantID to the absolute sub-manifest
	// URL (HLS) or base MPD + Representation selector (DASH). nil entries
	// mean the master manifest / MPD should be used directly.
	InternalVariantManifests map[string]string `json:"-"`

	// InternalYTDLPSource contains only backend re-resolution context. It is
	// never serialized to the frontend because yt-dlp selectors and browser
	// authority are not user-controlled download parameters.
	InternalYTDLPSource *YTDLPSource `json:"-"`
}

// YTDLPSource is backend-only context used to resolve short-lived media URLs
// again when a download task starts. The JSON tags make accidental Wails
// serialization fail closed even if this struct is embedded in a response.
type YTDLPSource struct {
	PageURL        string `json:"-"`
	FormatSelector string `json:"-"`
	Browser        string `json:"-"`
	Profile        string `json:"-"`
}

// MediaVariant describes a selectable rendition in an HLS or DASH manifest.
type MediaVariant struct {
	ID               string `json:"id"`
	Label            string `json:"label"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	Bandwidth        int64  `json:"bandwidth,omitempty"`
	HasVideo         bool   `json:"hasVideo"`
	HasAudio         bool   `json:"hasAudio"`
	AudioDescription string `json:"audioDescription"`

	// InternalFormatSelector is the yt-dlp format ID/selector used by the
	// backend when it re-resolves this variant. It is not a frontend contract.
	InternalFormatSelector string `json:"-"`
}

// ManifestVariant is the analyzer-internal intermediate representation of a
// selectable rendition. It carries information needed to build the final
// MediaCandidate + MediaVariant pair but is deliberately not shared with the
// frontend: raw segment URLs and codec descriptors must stay inside the
// analyzer layer.
type ManifestVariant struct {
	ID             string
	Label          string
	Width          int
	Height         int
	Bandwidth      int64
	HasVideo       bool
	HasAudio       bool
	Codecs         string
	SegmentURL     string
	BaseSegment    string
	SubManifestURL string
}

// ManifestAudioTrack is an audio-only variant described by EXT-X-MEDIA (HLS)
// or a DASH AdaptationSet/AudioChannelConfiguration pair.
type ManifestAudioTrack struct {
	ID         string
	Label      string
	Language   string
	Bandwidth  int64
	HasAudio   bool
	Codecs     string
	SegmentURL string
}

// DownloadRequest refers to an analyzed item by opaque IDs. The backend resolves
// the original source URL from its analysis session rather than trusting a URL
// supplied by the frontend.
type DownloadRequest struct {
	AnalysisID string          `json:"analysisId"`
	MediaID    string          `json:"mediaId"`
	VariantID  string          `json:"variantId,omitempty"`
	OutputPath string          `json:"outputPath"`
	Profile    DownloadProfile `json:"profile"`
}

// TaskState is the stable lifecycle state exposed to the frontend.
type TaskState string

const (
	TaskQueued      TaskState = "queued"
	TaskPreparing   TaskState = "preparing"
	TaskDownloading TaskState = "downloading"
	TaskMerging     TaskState = "merging"
	TaskTranscoding TaskState = "transcoding"
	TaskCompleted   TaskState = "completed"
	TaskCanceled    TaskState = "canceled"
	TaskFailed      TaskState = "failed"
)

// DownloadTask is a frontend-safe snapshot of a task. A nil Progress means the
// total size or duration is unknown; consumers must not render it as 100%.
type DownloadTask struct {
	ID                  string          `json:"id"`
	Title               string          `json:"title"`
	AnalysisID          string          `json:"analysisId,omitempty"`
	MediaID             string          `json:"mediaId,omitempty"`
	VariantID           string          `json:"variantId,omitempty"`
	Profile             DownloadProfile `json:"profile"`
	State               TaskState       `json:"state"`
	Phase               string          `json:"phase,omitempty"`
	Attempt             int             `json:"attempt"`
	Progress            *float64        `json:"progress,omitempty"`
	SpeedBytesPerSecond int64           `json:"speedBytesPerSecond,omitempty"`
	SizeBytes           *int64          `json:"sizeBytes,omitempty"`
	OutputPath          string          `json:"outputPath,omitempty"`
	ErrorCode           string          `json:"errorCode,omitempty"`
	ErrorMessage        string          `json:"errorMessage,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	StartedAt           *time.Time      `json:"startedAt,omitempty"`
	CompletedAt         *time.Time      `json:"completedAt,omitempty"`
}

// --- Wails event names ---

const (
	// EventAnalysisUpdate is emitted when the analysis lifecycle changes.
	// Payload: AnalysisEvent
	EventAnalysisUpdate = "analysis:update"

	// EventTaskUpdate is emitted on any task state or progress change.
	// Payload: TaskEvent
	EventTaskUpdate = "task:update"
)

// --- Event payloads (versioned) ---

// EventVersion is the current version of event payload structures. Any breaking
// change to the fields must bump this value so the frontend can handle older
// payloads gracefully if needed.
const EventVersion = 1

// AnalysisPhase describes the lifecycle of a single analysis.
type AnalysisPhase string

const (
	AnalysisRunning   AnalysisPhase = "running"
	AnalysisCompleted AnalysisPhase = "completed"
	AnalysisCanceled  AnalysisPhase = "canceled"
	AnalysisFailed    AnalysisPhase = "failed"
)

// AnalysisEvent is the payload of EventAnalysisUpdate. It carries the full
// AnalysisResult on completion or failure context on early exit.
type AnalysisEvent struct {
	Version      int             `json:"v"`
	AnalysisID   string          `json:"analysisId"`
	Phase        AnalysisPhase   `json:"phase"`
	PageTitle    string          `json:"pageTitle,omitempty"`
	CandidateCnt int             `json:"candidateCount,omitempty"`
	Result       *AnalysisResult `json:"result"`
	ErrorCode    string          `json:"errorCode,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
}

// TaskEvent is the payload of EventTaskUpdate. It embeds the task snapshot so
// the frontend can render the full state from a single event.
type TaskEvent struct {
	Version int          `json:"v"`
	Task    DownloadTask `json:"task"`
}

// --- Error codes (base definitions; service-specific codes live in their packages) ---

// ErrorCode is a stable, machine-readable error identifier. Values follow the
// pattern "domain.specific" (e.g. "analyzer.url_parse", "download.ffmpeg_exit").
// Services register their own codes; this type is the shared contract.
type ErrorCode string

const (
	// ErrCodeOK represents the absence of an error; used for completeness.
	ErrCodeOK ErrorCode = ""

	// ErrCodeUnknown is the fallback when a service did not provide a structured code.
	ErrCodeUnknown ErrorCode = "unknown"

	// ErrCodeCanceled indicates the user or system canceled the operation.
	ErrCodeCanceled ErrorCode = "canceled"

	// ErrCodeTimeout indicates the operation exceeded its deadline.
	ErrCodeTimeout ErrorCode = "timeout"
)
