package analyzer

import (
	"net/url"
	"strings"
	"testing"

	"videodl/internal/media"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid URL %q: %v", raw, err)
	}
	return u
}

func sampleMultivariantHLS() string {
	return `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=854x480,CODECS="avc1.42e01e,mp4a.40.2"
low/main.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1600000,RESOLUTION=1280x720,CODECS="avc1.42e01f,mp4a.40.2"
medium/main.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=4000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
high/main.m3u8
`
}

func sampleAudioVideoSeparateHLS() string {
	return `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",DEFAULT=YES,URI="audio/eng.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Spanish",LANGUAGE="es",URI="audio/esp.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720,CODECS="avc1.42e01f",AUDIO="audio"
video/720p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080,CODECS="avc1.640028",AUDIO="audio"
video/1080p.m3u8
`
}

func sampleSingleVariantHLS() string {
	return `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:9.009,
segment001.ts
#EXTINF:9.009,
segment002.ts
#EXT-X-ENDLIST
`
}

func sampleDRMHLS() string {
	return `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1280x720,CODECS="avc1.42e01f,mp4a.40.2"
clear/main.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1280x720,CODECS="avc1.42e01f,mp4a.40.2"
widevine/main.m3u8
#EXT-X-KEY:METHOD=SAMPLE-AES,URI="https://example.com/key"
`
}

func sampleInvalidHLS() string {
	return `this is not a playlist
just some random text
without any markers
`
}

func sampleDASHMultirep() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011"
     type="static"
     mediaPresentationDuration="PT30S"
     minBufferTime="PT2S"
     profiles="urn:mpeg:dash:profile:isoff-on-demand:2011">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <Representation id="v1" bandwidth="800000" width="854" height="480" codecs="avc1.42E01E">
        <SegmentList duration="30">
          <SegmentURL media="video/480p.mp4"/>
        </SegmentList>
      </Representation>
      <Representation id="v2" bandwidth="1600000" width="1280" height="720" codecs="avc1.42E01F">
        <SegmentList duration="30">
          <SegmentURL media="video/720p.mp4"/>
        </SegmentList>
      </Representation>
      <Representation id="v3" bandwidth="4000000" width="1920" height="1080" codecs="avc1.640028">
        <SegmentList duration="30">
          <SegmentURL media="video/1080p.mp4"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4" contentType="audio">
      <Representation id="a1" bandwidth="128000" codecs="mp4a.40.2">
        <SegmentList duration="30">
          <SegmentURL media="audio/aac.mp4"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>
`
}

func sampleDASHDRM() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT30S">
  <Period>
    <ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc">
      <cenc:pssh xmlns:cenc="urn:mpeg:cenc:2013">AAAAB3NzaC1yc2EAAAADAQ</cenc:pssh>
    </ContentProtection>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <Representation id="v1" bandwidth="2000000" width="1280" height="720"/>
    </AdaptationSet>
  </Period>
</MPD>
`
}

func sampleDASHInvalid() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<notMPD>
  <something>wrong</something>
</notMPD>
`
}

func TestDetectManifestType(t *testing.T) {
	cases := []struct {
		name string
		body string
		want media.SourceType
	}{
		{"HLS multivariant", sampleMultivariantHLS(), media.SourceHLS},
		{"HLS single variant", sampleSingleVariantHLS(), media.SourceHLS},
		{"HLS with DRM key", sampleDRMHLS(), media.SourceHLS},
		{"DASH", sampleDASHMultirep(), media.SourceDASH},
		{"DASH with DRM", sampleDASHDRM(), media.SourceDASH},
		{"invalid HLS", sampleInvalidHLS(), ""},
		{"invalid DASH", sampleDASHInvalid(), ""},
		{"empty", "", ""},
		{"random text", "hello world", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectManifestType(c.body)
			if got != c.want {
				t.Errorf("DetectManifestType() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseHLS_Multivariant(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/master.m3u8")
	res, err := ParseManifest([]byte(sampleMultivariantHLS()), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if res.DRM {
		t.Error("expected DRM = false for clear HLS")
	}
	if res.SourceType != media.SourceHLS {
		t.Errorf("SourceType = %q, want hls", res.SourceType)
	}
	if len(res.Variants) != 3 {
		t.Fatalf("len(Variants) = %d, want 3", len(res.Variants))
	}

	bwMap := map[int64]media.ManifestVariant{}
	for _, v := range res.Variants {
		bwMap[v.Bandwidth] = v
	}
	for _, bw := range []int64{800000, 1600000, 4000000} {
		v, ok := bwMap[bw]
		if !ok {
			t.Fatalf("missing bandwidth %d", bw)
		}
		if !v.HasVideo {
			t.Errorf("bandwidth %d: HasVideo = false", bw)
		}
		if !v.HasAudio {
			t.Errorf("bandwidth %d: HasAudio = false", bw)
		}
	}
	low := bwMap[800000]
	if low.Width != 854 || low.Height != 480 {
		t.Errorf("low variant resolution = %dx%d, want 854x480", low.Width, low.Height)
	}
}

func TestParseHLS_AudioVideoSeparate(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/master.m3u8")
	res, err := ParseManifest([]byte(sampleAudioVideoSeparateHLS()), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if res.DRM {
		t.Error("expected DRM = false")
	}
	if len(res.Variants) != 2 {
		t.Fatalf("len(Variants) = %d, want 2", len(res.Variants))
	}
	for _, v := range res.Variants {
		if !v.HasVideo {
			t.Errorf("variant %q: HasVideo = false", v.Label)
		}
	}

	audioTracks := map[string]bool{}
	for _, a := range res.AudioTracks {
		audioTracks[a.Language] = true
	}
	if !audioTracks["en"] || !audioTracks["es"] {
		t.Errorf("expected audio tracks en and es, got %v", audioTracks)
	}
}

func TestParseHLS_SingleVariant(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/plain.m3u8")
	res, err := ParseManifest([]byte(sampleSingleVariantHLS()), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if res.DRM {
		t.Error("expected DRM = false")
	}
	if len(res.Variants) != 1 {
		t.Fatalf("len(Variants) = %d, want 1", len(res.Variants))
	}
	v := res.Variants[0]
	if !v.HasVideo || !v.HasAudio {
		t.Errorf("single variant flags: video=%v audio=%v", v.HasVideo, v.HasAudio)
	}
}

func TestParseHLS_DRMDetected(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/protected.m3u8")
	res, err := ParseManifest([]byte(sampleDRMHLS()), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if !res.DRM {
		t.Error("expected DRM = true")
	}
	if !strings.Contains(res.DRMReason, "SAMPLE-AES") {
		t.Errorf("DRMReason = %q, should mention SAMPLE-AES", res.DRMReason)
	}
}

func TestParseHLS_RelativeURIs(t *testing.T) {
	body := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=640x360
subdir/playlist.m3u8
`
	base := mustURL(t, "https://cdn.example.com/a/b/master.m3u8")
	res, err := ParseManifest([]byte(body), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if len(res.Variants) != 1 {
		t.Fatalf("len(Variants) = %d", len(res.Variants))
	}
	want := "https://cdn.example.com/a/b/subdir/playlist.m3u8"
	if res.Variants[0].SegmentURL != want {
		t.Errorf("SegmentURL = %q, want %q", res.Variants[0].SegmentURL, want)
	}
}

func TestParseDASH_MultiRepresentation(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/mpd/manifest.mpd")
	res, err := ParseManifest([]byte(sampleDASHMultirep()), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if res.SourceType != media.SourceDASH {
		t.Errorf("SourceType = %q, want dash", res.SourceType)
	}
	if res.DRM {
		t.Error("expected DRM = false")
	}

	if len(res.Variants) != 3 {
		t.Fatalf("len(video Variants) = %d, want 3 video representations", len(res.Variants))
	}

	bwMap := map[int64]media.ManifestVariant{}
	for _, v := range res.Variants {
		bwMap[v.Bandwidth] = v
	}
	for _, bw := range []int64{800000, 1600000, 4000000} {
		if _, ok := bwMap[bw]; !ok {
			t.Errorf("missing video representation bandwidth %d", bw)
		}
	}

	low := bwMap[800000]
	if low.Width != 854 || low.Height != 480 {
		t.Errorf("480p variant resolution = %dx%d, want 854x480", low.Width, low.Height)
	}

	if len(res.AudioTracks) != 1 {
		t.Fatalf("len(AudioTracks) = %d, want 1", len(res.AudioTracks))
	}
	a := res.AudioTracks[0]
	if !a.HasAudio || a.Bandwidth != 128000 {
		t.Errorf("audio variant mismatch: %+v", a)
	}
}

func TestParseDASH_DRMDetected(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/protected/manifest.mpd")
	res, err := ParseManifest([]byte(sampleDASHDRM()), base)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if !res.DRM {
		t.Error("expected DRM = true for DASH with ContentProtection")
	}
	if !strings.Contains(res.DRMReason, "ContentProtection") && !strings.Contains(res.DRMReason, "cenc") {
		t.Errorf("DRMReason = %q, should mention protection scheme", res.DRMReason)
	}
}

func TestParseDASH_InvalidReturnsError(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/manifest.mpd")
	_, err := ParseManifest([]byte(sampleDASHInvalid()), base)
	if err == nil {
		t.Error("expected error for invalid DASH XML")
	}
}

func TestParseManifest_InvalidBodyReturnsError(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/manifest")
	_, err := ParseManifest([]byte(sampleInvalidHLS()), base)
	if err == nil {
		t.Error("expected error for non-manifest body")
	}
}

func TestParseManifest_EmptyBody(t *testing.T) {
	base := mustURL(t, "https://cdn.example.com/manifest")
	_, err := ParseManifest(nil, base)
	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestParseManifest_NilBaseURL(t *testing.T) {
	body := sampleMultivariantHLS()
	_, err := ParseManifest([]byte(body), nil)
	if err == nil {
		t.Error("expected error for nil base URL")
	}
}
