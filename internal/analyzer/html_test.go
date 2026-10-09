package analyzer

import (
	"net/url"
	"testing"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid base URL %q: %v", raw, err)
	}
	return u
}

func TestExtractMedia_SingleVideoSrc(t *testing.T) {
	base := mustParse(t, "https://example.com/page/index.html")
	html := `
<!doctype html>
<html><head><title>My Page</title></head>
<body>
  <video src="video.mp4" controls></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if info.PageTitle != "My Page" {
		t.Errorf("PageTitle = %q, want %q", info.PageTitle, "My Page")
	}
	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(info.Candidates))
	}
	c := info.Candidates[0]
	if c.DisplayURL != "https://example.com/page/video.mp4" {
		t.Errorf("DisplayURL = %q, want absolute resolved URL", c.DisplayURL)
	}
	if c.SourceTag != "video" {
		t.Errorf("SourceTag = %q, want %q", c.SourceTag, "video")
	}
}

func TestExtractMedia_VideoWithSourceChildren(t *testing.T) {
	base := mustParse(t, "https://example.com/media/player.html")
	html := `
<html><head><title>Demo</title></head>
<body>
  <video controls>
    <source src="clip.webm" type="video/webm">
    <source src="clip.mp4" type="video/mp4">
  </video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 2 {
		t.Fatalf("len(Candidates) = %d, want 2", len(info.Candidates))
	}
	want := map[string]string{
		"https://example.com/media/clip.webm": "video/webm",
		"https://example.com/media/clip.mp4":  "video/mp4",
	}
	for _, c := range info.Candidates {
		wantType, ok := want[c.DisplayURL]
		if !ok {
			t.Errorf("unexpected candidate URL: %s", c.DisplayURL)
			continue
		}
		if c.MIMEType != wantType {
			t.Errorf("MIMEType for %s = %q, want %q", c.DisplayURL, c.MIMEType, wantType)
		}
		if c.SourceTag != "source" {
			t.Errorf("SourceTag = %q, want %q", c.SourceTag, "source")
		}
	}
}

func TestExtractMedia_MultipleVideosAndAudio(t *testing.T) {
	base := mustParse(t, "https://cdn.example.com/gallery/")
	html := `
<html><head><title>Gallery</title></head>
<body>
  <video src="a.mp4"></video>
  <audio src="podcast.mp3"></audio>
  <video><source src="b.webm"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 3 {
		t.Fatalf("len(Candidates) = %d, want 3", len(info.Candidates))
	}
	urls := map[string]bool{}
	for _, c := range info.Candidates {
		urls[c.DisplayURL] = true
	}
	for _, want := range []string{
		"https://cdn.example.com/gallery/a.mp4",
		"https://cdn.example.com/gallery/podcast.mp3",
		"https://cdn.example.com/gallery/b.webm",
	} {
		if !urls[want] {
			t.Errorf("missing expected candidate: %s", want)
		}
	}
}

func TestExtractMedia_RelativeWithBaseTag(t *testing.T) {
	base := mustParse(t, "https://page.example.com/article/index.html")
	html := `
<html>
<head>
  <base href="https://cdn.example.com/videos/">
  <title>Article</title>
</head>
<body>
  <video src="intro.mp4"></video>
</body>
</html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://cdn.example.com/videos/intro.mp4" {
		t.Errorf("DisplayURL = %q, want base-tag-resolved URL", info.Candidates[0].DisplayURL)
	}
}

func TestExtractMedia_AbsoluteURLsKeptAsIs(t *testing.T) {
	base := mustParse(t, "https://example.com/page.html")
	html := `
<html><body>
  <video src="https://other.com/video.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://other.com/video.mp4" {
		t.Errorf("DisplayURL = %q, want absolute URL unchanged", info.Candidates[0].DisplayURL)
	}
}

func TestExtractMedia_DuplicateURLsDeduplicated(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `
<html><body>
  <video src="clip.mp4"></video>
  <video><source src="clip.mp4"></video>
  <video src="./clip.mp4"></video>
  <video src="https://example.com/clip.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1 (duplicates should be merged)", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://example.com/clip.mp4" {
		t.Errorf("DisplayURL = %q", info.Candidates[0].DisplayURL)
	}
}

func TestExtractMedia_NoMediaReturnsEmpty(t *testing.T) {
	base := mustParse(t, "https://example.com/empty.html")
	html := `
<html><head><title>Just a page</title></head>
<body><p>no media here</p><a href="other.html">link</a></body></html>`

	info := ExtractMediaFromHTML(html, base)

	if info.PageTitle != "Just a page" {
		t.Errorf("PageTitle = %q", info.PageTitle)
	}
	if len(info.Candidates) != 0 {
		t.Errorf("len(Candidates) = %d, want 0", len(info.Candidates))
	}
}

func TestExtractMedia_MalformedHTML(t *testing.T) {
	base := mustParse(t, "https://example.com/broken.html")
	html := `
<html><head><title>Broken</title></head>
<body>
  <video src="a.mp4"
  <source src="b.webm" type=video/webm>
  <<< garbage >>>
</body>`

	info := ExtractMediaFromHTML(html, base)

	if info.PageTitle != "Broken" {
		t.Errorf("PageTitle = %q, want %q", info.PageTitle, "Broken")
	}
	if len(info.Candidates) < 1 {
		t.Errorf("expected at least 1 candidate from malformed HTML, got %d", len(info.Candidates))
	}
}

func TestExtractMedia_EmptyInput(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	info := ExtractMediaFromHTML("", base)

	if info.PageTitle != "" {
		t.Errorf("PageTitle = %q, want empty", info.PageTitle)
	}
	if len(info.Candidates) != 0 {
		t.Errorf("len(Candidates) = %d, want 0", len(info.Candidates))
	}
}

func TestExtractMedia_OGVideoMeta(t *testing.T) {
	base := mustParse(t, "https://example.com/post/123")
	html := `
<html><head>
  <title>Post</title>
  <meta property="og:video" content="https://cdn.example.com/og.mp4">
  <meta property="og:video:type" content="video/mp4">
</head><body></body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(info.Candidates))
	}
	c := info.Candidates[0]
	if c.DisplayURL != "https://cdn.example.com/og.mp4" {
		t.Errorf("DisplayURL = %q", c.DisplayURL)
	}
	if c.SourceTag != "meta" {
		t.Errorf("SourceTag = %q, want %q", c.SourceTag, "meta")
	}
}

func TestExtractMedia_JSONLDVideoObject(t *testing.T) {
	base := mustParse(t, "https://example.com/article")
	html := `
<html><head><title>Article</title>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "VideoObject",
  "name": "My Video",
  "contentUrl": "https://videos.example.com/ld.mp4"
}
</script></head><body></body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://videos.example.com/ld.mp4" {
		t.Errorf("DisplayURL = %q", info.Candidates[0].DisplayURL)
	}
	if info.Candidates[0].SourceTag != "jsonld" {
		t.Errorf("SourceTag = %q, want %q", info.Candidates[0].SourceTag, "jsonld")
	}
}

func TestExtractMedia_VideoPosterNotExtracted(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `
<html><body>
  <video poster="cover.jpg" src="clip.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	urls := map[string]bool{}
	for _, c := range info.Candidates {
		urls[c.DisplayURL] = true
	}
	if urls["https://example.com/cover.jpg"] {
		t.Error("poster image should not be extracted as media candidate")
	}
	if !urls["https://example.com/clip.mp4"] {
		t.Error("video src should be extracted")
	}
}

func TestExtractMedia_IgnoresJavaScript(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `
<html><head><title>Dynamic</title></head>
<body>
  <script>
    var video = document.createElement('video');
    video.src = 'dynamic.mp4';
  </script>
  <video src="static.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1 (script content must not be parsed)", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://example.com/static.mp4" {
		t.Errorf("should only extract static.mp4, got %s", info.Candidates[0].DisplayURL)
	}
}

func TestExtractMedia_ResolvesWithDotSegments(t *testing.T) {
	base := mustParse(t, "https://example.com/a/b/c.html")
	html := `<video src="../x.mp4"></video>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d", len(info.Candidates))
	}
	got := info.Candidates[0].DisplayURL
	want := "https://example.com/a/x.mp4"
	if got != want {
		t.Errorf("DisplayURL = %q, want %q", got, want)
	}
}

func TestExtractMedia_SkipsInvalidURLs(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `
<html><body>
  <video src="https://"></video>
  <video src="not a url"></video>
  <video src="javascript:alert(1)"></video>
  <video src="https://valid.com/good.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1 after skipping invalid", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://valid.com/good.mp4" {
		t.Errorf("DisplayURL = %q", info.Candidates[0].DisplayURL)
	}
}

func TestExtractMedia_DisplayURLStripsQueryAndFragment(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `<video src="video.mp4?token=abc#part2"></video>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d", len(info.Candidates))
	}
	c := info.Candidates[0]
	if c.DisplayURL != "https://example.com/video.mp4" {
		t.Errorf("DisplayURL = %q, want query and fragment stripped", c.DisplayURL)
	}
	if c.URL != "https://example.com/video.mp4?token=abc#part2" {
		t.Errorf("URL = %q, want original with query/fragment preserved", c.URL)
	}
}

func TestExtractMedia_IgnoresDataURIs(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `
<html><body>
  <video src="data:video/mp4;base64,AAA"></video>
  <video src="real.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if len(info.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(info.Candidates))
	}
	if info.Candidates[0].DisplayURL != "https://example.com/real.mp4" {
		t.Errorf("unexpected candidate: %s", info.Candidates[0].DisplayURL)
	}
}

func TestExtractMedia_TitleFromMetaFallback(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	html := `
<html><head>
  <meta property="og:title" content="OG Title">
</head><body>
  <video src="v.mp4"></video>
</body></html>`

	info := ExtractMediaFromHTML(html, base)

	if info.PageTitle != "OG Title" {
		t.Errorf("PageTitle = %q, want og:title fallback", info.PageTitle)
	}
}
