package media

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSourceTypeConstants(t *testing.T) {
	if SourceDirect != "direct" {
		t.Errorf("SourceDirect = %q, want %q", SourceDirect, "direct")
	}
	if SourceHLS != "hls" {
		t.Errorf("SourceHLS = %q, want %q", SourceHLS, "hls")
	}
	if SourceDASH != "dash" {
		t.Errorf("SourceDASH = %q, want %q", SourceDASH, "dash")
	}
}

func TestDownloadProfileConstants(t *testing.T) {
	if ProfileOriginal != "original" {
		t.Errorf("ProfileOriginal = %q, want %q", ProfileOriginal, "original")
	}
	if ProfileMP4 != "mp4-h264-aac" {
		t.Errorf("ProfileMP4 = %q, want %q", ProfileMP4, "mp4-h264-aac")
	}
}

func TestTaskStateConstants(t *testing.T) {
	want := []TaskState{TaskQueued, TaskPreparing, TaskDownloading, TaskMerging, TaskTranscoding, TaskCompleted, TaskCanceled, TaskFailed}
	seen := make(map[TaskState]bool)
	for _, s := range want {
		if seen[s] {
			t.Errorf("duplicate TaskState constant: %q", s)
		}
		seen[s] = true
	}
}

func TestAnalysisPhaseConstants(t *testing.T) {
	want := []AnalysisPhase{AnalysisRunning, AnalysisCompleted, AnalysisCanceled, AnalysisFailed}
	seen := make(map[AnalysisPhase]bool)
	for _, p := range want {
		if seen[p] {
			t.Errorf("duplicate AnalysisPhase constant: %q", p)
		}
		seen[p] = true
	}
}

func TestErrorCodeConstants(t *testing.T) {
	if ErrCodeOK != "" {
		t.Errorf("ErrCodeOK = %q, want empty", ErrCodeOK)
	}
	if ErrCodeUnknown != "unknown" {
		t.Errorf("ErrCodeUnknown = %q, want %q", ErrCodeUnknown, "unknown")
	}
	if ErrCodeCanceled != "canceled" {
		t.Errorf("ErrCodeCanceled = %q, want %q", ErrCodeCanceled, "canceled")
	}
	if ErrCodeTimeout != "timeout" {
		t.Errorf("ErrCodeTimeout = %q, want %q", ErrCodeTimeout, "timeout")
	}
}

func TestEventNameConstants(t *testing.T) {
	if EventAnalysisUpdate != "analysis:update" {
		t.Errorf("EventAnalysisUpdate = %q, want %q", EventAnalysisUpdate, "analysis:update")
	}
	if EventTaskUpdate != "task:update" {
		t.Errorf("EventTaskUpdate = %q, want %q", EventTaskUpdate, "task:update")
	}
}

func TestEventVersion(t *testing.T) {
	if EventVersion != 1 {
		t.Errorf("EventVersion = %d, want 1", EventVersion)
	}
}

func TestAnalysisResult_JSONRoundTrip(t *testing.T) {
	dur := 123.5
	size := int64(1024000)
	result := AnalysisResult{
		ID:        "ana_abc123",
		PageTitle: "Sample Page",
		Candidates: []MediaCandidate{
			{
				ID:              "med_001",
				Title:           "Big Buck Bunny",
				DisplayURL:      "https://example.com/video",
				SourceType:      SourceDirect,
				Format:          "mp4",
				Width:           1920,
				Height:          1080,
				DurationSeconds: &dur,
				SizeBytes:       &size,
				HasVideo:        true,
				HasAudio:        true,
			},
		},
		Warnings: []string{"Some pages may not be fully supported."},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded AnalysisResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.ID != result.ID {
		t.Errorf("ID mismatch: got %q want %q", decoded.ID, result.ID)
	}
	if decoded.PageTitle != result.PageTitle {
		t.Errorf("PageTitle mismatch")
	}
	if len(decoded.Candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(decoded.Candidates))
	}
	if decoded.Candidates[0].DurationSeconds == nil || *decoded.Candidates[0].DurationSeconds != dur {
		t.Errorf("DurationSeconds round-trip failed")
	}
	if decoded.Candidates[0].SizeBytes == nil || *decoded.Candidates[0].SizeBytes != size {
		t.Errorf("SizeBytes round-trip failed")
	}
}

func TestAnalysisResult_JSONFieldNames(t *testing.T) {
	dur := 60.0
	result := AnalysisResult{
		ID:        "ana_1",
		PageTitle: "T",
		Candidates: []MediaCandidate{
			{
				ID:              "med_1",
				Title:           "c",
				SourceType:      SourceHLS,
				DurationSeconds: &dur,
				Variants: []MediaVariant{
					{ID: "v1", Label: "720p", Width: 1280, Height: 720, Bandwidth: 2500000, HasVideo: true, HasAudio: true},
				},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)

	expectedKeys := []string{
		`"id":"ana_1"`,
		`"pageTitle":"T"`,
		`"sourceType":"hls"`,
		`"durationSeconds":60`,
		`"variants"`,
		`"bandwidth":2500000`,
		`"audioDescription"`,
	}
	for _, key := range expectedKeys {
		if !strings.Contains(s, key) {
			t.Errorf("JSON missing expected key/substring: %s", key)
		}
	}
}

func TestMediaCandidate_UnsupportedField(t *testing.T) {
	c := MediaCandidate{
		ID:          "med_x",
		SourceType:  SourceDASH,
		HasVideo:    true,
		HasAudio:    true,
		Unsupported: "DRM protected",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"unsupported":"DRM protected"`) {
		t.Errorf("Unsupported field not serialized correctly: %s", string(data))
	}
}

func TestDownloadRequest_JSONTags(t *testing.T) {
	req := DownloadRequest{
		AnalysisID: "ana_1",
		MediaID:    "med_1",
		VariantID:  "v1",
		OutputPath: "/tmp/out.mp4",
		Profile:    ProfileMP4,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, key := range []string{`"analysisId":"ana_1"`, `"mediaId":"med_1"`, `"variantId":"v1"`, `"outputPath":"/tmp/out.mp4"`, `"profile":"mp4-h264-aac"`} {
		if !strings.Contains(s, key) {
			t.Errorf("missing key %s in %s", key, s)
		}
	}
}

func TestDownloadTask_NilProgressOmitted(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := DownloadTask{
		ID:         "task_1",
		Title:      "T",
		State:      TaskDownloading,
		CreatedAt:  ts,
		Attempt:    1,
		Profile:    ProfileOriginal,
		AnalysisID: "ana_1",
		MediaID:    "med_1",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	if strings.Contains(s, `"progress"`) {
		t.Errorf("nil Progress should be omitted, got: %s", s)
	}
	if strings.Contains(s, `"startedAt"`) {
		t.Errorf("nil StartedAt should be omitted, got: %s", s)
	}
	if strings.Contains(s, `"completedAt"`) {
		t.Errorf("nil CompletedAt should be omitted, got: %s", s)
	}
	if strings.Contains(s, `"errorCode"`) {
		t.Errorf("empty ErrorCode should be omitted, got: %s", s)
	}
	if strings.Contains(s, `"errorMessage"`) {
		t.Errorf("empty ErrorMessage should be omitted, got: %s", s)
	}
	if !strings.Contains(s, `"createdAt"`) {
		t.Errorf("CreatedAt should always present, got: %s", s)
	}
	if !strings.Contains(s, `"analysisId":"ana_1"`) {
		t.Errorf("AnalysisID should be present, got: %s", s)
	}
	if !strings.Contains(s, `"attempt":1`) {
		t.Errorf("Attempt should be present, got: %s", s)
	}
}

func TestDownloadTask_WithProgress(t *testing.T) {
	prog := 42.5
	task := DownloadTask{
		ID:       "task_2",
		Title:    "T2",
		State:    TaskDownloading,
		Progress: &prog,
	}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"progress":42.5`) {
		t.Errorf("progress should serialize, got: %s", string(data))
	}
}

func TestAnalysisEvent_JSONTags(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	event := AnalysisEvent{
		Version:    EventVersion,
		AnalysisID: "ana_1",
		Phase:      AnalysisRunning,
		PageTitle:  "Page",
	}
	_ = ts

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	for _, key := range []string{`"v":1`, `"analysisId":"ana_1"`, `"phase":"running"`, `"pageTitle":"Page"`} {
		if !strings.Contains(s, key) {
			t.Errorf("missing key %s in %s", key, s)
		}
	}
}

func TestAnalysisEvent_FailedPhase(t *testing.T) {
	event := AnalysisEvent{
		Version:      EventVersion,
		AnalysisID:   "ana_fail",
		Phase:        AnalysisFailed,
		ErrorCode:    "analyzer.network",
		ErrorMessage: "connection refused",
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"phase":"failed"`) {
		t.Error("phase should be 'failed'")
	}
	if !strings.Contains(s, `"errorCode":"analyzer.network"`) {
		t.Error("errorCode should be present")
	}
	if !strings.Contains(s, `"result"`) {
		t.Error("result key should be present even when nil (to distinguish from omit)")
	}
}

func TestTaskEvent_JSONTags(t *testing.T) {
	task := DownloadTask{
		ID:        "task_1",
		State:     TaskQueued,
		Attempt:   1,
		Profile:   ProfileOriginal,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	event := TaskEvent{
		Version: EventVersion,
		Task:    task,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	if !strings.Contains(s, `"v":1`) {
		t.Errorf("missing version in %s", s)
	}
	if !strings.Contains(s, `"task"`) {
		t.Errorf("missing task wrapper in %s", s)
	}
	if !strings.Contains(s, `"state":"queued"`) {
		t.Errorf("missing task state in %s", s)
	}
	if !strings.Contains(s, `"profile":"original"`) {
		t.Errorf("missing profile in %s", s)
	}
}

func TestOptionalInt64Serialization(t *testing.T) {
	size := int64(12345)
	c := MediaCandidate{
		ID:         "med_size",
		SourceType: SourceDirect,
		HasVideo:   true,
		HasAudio:   true,
		SizeBytes:  &size,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"sizeBytes":12345`) {
		t.Errorf("optional int64 should serialize correctly, got: %s", string(data))
	}

	c2 := MediaCandidate{
		ID:         "med_nosize",
		SourceType: SourceDirect,
		HasVideo:   true,
		HasAudio:   true,
	}
	data2, err := json.Marshal(c2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data2), `"sizeBytes"`) {
		t.Errorf("nil SizeBytes should be omitted, got: %s", string(data2))
	}
}

func TestNilDurationSerialization(t *testing.T) {
	c := MediaCandidate{
		ID:         "med_nodur",
		SourceType: SourceDirect,
		HasVideo:   true,
		HasAudio:   true,
	}
	data, _ := json.Marshal(c)
	if strings.Contains(string(data), `"durationSeconds"`) {
		t.Errorf("nil DurationSeconds should be omitted, got: %s", string(data))
	}
}
