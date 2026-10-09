.PHONY: ffmpeg-fetch ffmpeg-verify ffmpeg-clean ffmpeg-manifest-test test-ffmpeg

ffmpeg-fetch:
	@echo "Fetching FFmpeg binaries for all target platforms..."
	@bash scripts/fetch_ffmpeg.sh

ffmpeg-verify:
	@echo "Verifying FFmpeg binaries in place..."
	@bash scripts/verify_ffmpeg.sh

ffmpeg-clean:
	@echo "Removing fetched FFmpeg binaries..."
	rm -rf build/resources/ffmpeg/tools/windows-x64
	rm -rf build/resources/ffmpeg/tools/linux-x64
	rm -rf build/resources/ffmpeg/tools/darwin-x64
	rm -rf build/resources/ffmpeg/tools/darwin-arm64

ffmpeg-manifest-test:
	@bash scripts/test_ffmpeg_manifest.sh

test-ffmpeg:
	go test ./internal/ffmpeg/ -v -count=1
