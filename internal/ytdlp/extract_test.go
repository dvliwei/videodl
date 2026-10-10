package ytdlp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerExtract_DefaultFlagsAndStructuredFields(t *testing.T) {
	skipShellTestOnWindows(t)
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args")
	jsonFile := filepath.Join(tmp, "single.json")
	copyTestdata(t, "single.json", jsonFile)
	script := writeStub(t, `#!/bin/sh
printf '%s\n' "$@" > "$YTDLP_TEST_ARGS_FILE"
cat "$YTDLP_TEST_JSON_FILE"`)
	runner := NewRunner(RunConfig{
		BinaryPath: script,
		Env: []string{
			"YTDLP_TEST_ARGS_FILE=" + argsFile,
			"YTDLP_TEST_JSON_FILE=" + jsonFile,
		},
	})
	info, err := runner.Extract(context.Background(), ExtractRequest{
		URL:            "https://video.example/watch?id=1",
		FormatSelector: "bestvideo[height<=720]+bestaudio",
		ProxyURL:       "http://127.0.0.1:34567",
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "video-1" || info.Title != "Example video" || len(info.Formats) != 1 {
		t.Fatalf("info = %#v", info)
	}
	if info.Duration == nil || *info.Duration != 12.5 {
		t.Fatalf("duration = %#v", info.Duration)
	}
	args := readLines(t, argsFile)
	want := []string{
		"--dump-single-json", "--no-playlist", "--ignore-config", "--no-cookies-from-browser",
		"--proxy", "http://127.0.0.1:34567", "--format", "bestvideo[height<=720]+bestaudio", "--",
		"https://video.example/watch?id=1",
	}
	if len(args) != len(want) {
		t.Fatalf("argv = %#v, want %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestRunnerExtract_BrowserValuesStaySingleArguments(t *testing.T) {
	skipShellTestOnWindows(t)
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args")
	jsonFile := filepath.Join(tmp, "json")
	copyTestdata(t, "single.json", jsonFile)
	script := writeStub(t, `#!/bin/sh
printf '%s\n' "$@" > "$YTDLP_TEST_ARGS_FILE"
cat "$YTDLP_TEST_JSON_FILE"`)
	runner := NewRunner(RunConfig{
		BinaryPath: script,
		Env:        []string{"YTDLP_TEST_ARGS_FILE=" + argsFile, "YTDLP_TEST_JSON_FILE=" + jsonFile},
	})
	_, err := runner.Extract(context.Background(), ExtractRequest{
		URL: "https://video.example/watch?id=1",
		Browser: &BrowserSession{
			Browser: "chrome",
			Profile: "Profile 1; --ignore-config",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	args := readLines(t, argsFile)
	if !containsPair(args, "--cookies-from-browser", "chrome:Profile 1; --ignore-config") {
		t.Fatalf("browser argv = %#v", args)
	}
	if contains(args, "--no-cookies-from-browser") {
		t.Fatalf("browser mode must not also pass the default cookie-disabled flag: %#v", args)
	}
}

func TestRunnerExtract_RejectsInvalidJSON(t *testing.T) {
	skipShellTestOnWindows(t)
	script := writeStub(t, `#!/bin/sh
printf '%s' '{not-json'`)
	_, err := NewRunner(RunConfig{BinaryPath: script}).Extract(context.Background(), ExtractRequest{URL: "https://video.example"})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestRunnerExtract_RejectsPlaylist(t *testing.T) {
	skipShellTestOnWindows(t)
	tmp := t.TempDir()
	jsonFile := filepath.Join(tmp, "playlist.json")
	copyTestdata(t, "playlist.json", jsonFile)
	script := writeStub(t, `#!/bin/sh
cat "$YTDLP_TEST_JSON_FILE"`)
	_, err := NewRunner(RunConfig{BinaryPath: script, Env: []string{"YTDLP_TEST_JSON_FILE=" + jsonFile}}).Extract(context.Background(), ExtractRequest{URL: "https://video.example/list"})
	if err == nil || !strings.Contains(err.Error(), "playlist") {
		t.Fatalf("expected playlist rejection, got %v", err)
	}
}

func TestInfoJSON_NoDurationIsOptional(t *testing.T) {
	var info Info
	data := []byte(`{"id":"no-duration","title":"No duration","formats":[]}`)
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if info.Duration != nil {
		t.Fatalf("duration = %#v, want nil", info.Duration)
	}
}

func copyTestdata(t *testing.T, name, destination string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsPair(values []string, first, second string) bool {
	for i := 0; i+1 < len(values); i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}
