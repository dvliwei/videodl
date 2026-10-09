.PHONY: ffmpeg-fetch ffmpeg-verify ffmpeg-clean ffmpeg-manifest-test test-ffmpeg build-darwin build-windows build-linux wails-build package-bundle

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
	rm -f build/resources/ffmpeg/SHA256SUMS.txt

ffmpeg-manifest-test:
	@bash scripts/test_ffmpeg_manifest.sh

test-ffmpeg:
	go test ./internal/ffmpeg/ -v -count=1

wails-build:
	wails build -clean

package-bundle:
	@if [ -d build/bin/videodl.app/Contents ]; then \
		bash scripts/copy_ffmpeg_to_bundle.sh build/bin/videodl.app/Contents; \
	elif [ -d build/bin ]; then \
		bash scripts/copy_ffmpeg_to_bundle.sh build/bin; \
	fi

build-darwin: ffmpeg-fetch
	@echo "Building macOS app bundle..."
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh build/bin/videodl.app/Contents
	@bash scripts/package_ffmpeg.sh darwin-arm64
	@bash scripts/package_ffmpeg.sh darwin-x64

build-windows: ffmpeg-fetch
	@echo "Building Windows executable..."
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh build/bin
	@bash scripts/package_ffmpeg.sh windows-x64

build-linux: ffmpeg-fetch
	@echo "Building Linux executable..."
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh build/bin
	@bash scripts/package_ffmpeg.sh linux-x64

quick-build:
	wails build -clean && make package-bundle
