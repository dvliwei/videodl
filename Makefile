.PHONY: ffmpeg-fetch ffmpeg-verify ffmpeg-clean ffmpeg-manifest-test test-ffmpeg
.PHONY: yt-dlp-fetch yt-dlp-verify yt-dlp-clean yt-dlp-manifest-test
.PHONY: wails-build package-bundle sign-bundle build-darwin build-windows build-linux quick-build
.PHONY: host-platform

HOST_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_ARCH_RAW := $(shell uname -m)
ifeq ($(HOST_ARCH_RAW),x86_64)
  HOST_ARCH := amd64
else ifeq ($(HOST_ARCH_RAW),amd64)
  HOST_ARCH := amd64
else ifeq ($(HOST_ARCH_RAW),arm64)
  HOST_ARCH := arm64
else ifeq ($(HOST_ARCH_RAW),aarch64)
  HOST_ARCH := arm64
else
  HOST_ARCH := $(HOST_ARCH_RAW)
endif

ifeq ($(HOST_OS),darwin)
  export CGO_LDFLAGS := -framework UniformTypeIdentifiers -mmacosx-version-min=10.13
  ifeq ($(HOST_ARCH),arm64)
    HOST_PLATFORM := darwin-arm64
  else
    HOST_PLATFORM := darwin-x64
  endif
else ifeq ($(HOST_OS),windows)
  HOST_PLATFORM := windows-x64
else ifeq ($(HOST_OS),linux)
  HOST_PLATFORM := linux-x64
else
  HOST_PLATFORM := unknown
endif

BUNDLE_PLATFORM ?= $(HOST_PLATFORM)

ifeq ($(HOST_OS),darwin)
  BUNDLE_CONTENTS := build/bin/videodl.app/Contents
  BUNDLE_FFMPEG   := build/bin/videodl.app/Contents/Resources/ffmpeg
  APP_PATH        := build/bin/videodl.app
else
  BUNDLE_CONTENTS := build/bin
  BUNDLE_FFMPEG   := build/bin/resources/ffmpeg
  APP_PATH        :=
endif

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

yt-dlp-fetch:
	@echo "Fetching official yt-dlp standalone assets..."
	@bash scripts/fetch_yt_dlp.sh

yt-dlp-verify:
	@echo "Verifying official yt-dlp standalone assets..."
	@bash scripts/verify_yt_dlp.sh

yt-dlp-clean:
	@echo "Removing fetched yt-dlp assets..."
	rm -rf build/resources/yt-dlp/downloads
	rm -f build/resources/yt-dlp/licenses/*.upstream.txt

yt-dlp-manifest-test:
	@bash scripts/test_yt_dlp_manifest.sh

wails-build:
	wails build -clean

package-bundle:
	@echo "Host platform: $(HOST_PLATFORM), packaging for: $(BUNDLE_PLATFORM)"
	@if [ -d "$(BUNDLE_CONTENTS)" ]; then \
		bash scripts/copy_ffmpeg_to_bundle.sh "$(BUNDLE_CONTENTS)" "$(BUNDLE_PLATFORM)"; \
		bash scripts/copy_yt_dlp_to_bundle.sh "$(BUNDLE_CONTENTS)" "$(BUNDLE_PLATFORM)"; \
		bash scripts/package_ffmpeg.sh "$(BUNDLE_FFMPEG)"; \
	else \
		echo "package-bundle: bundle not found at $(BUNDLE_CONTENTS)"; \
		echo "run 'make wails-build' first"; \
		exit 1; \
	fi
ifeq ($(HOST_OS),darwin)
	@bash scripts/codesign_bundle.sh "$(APP_PATH)"
endif

sign-bundle:
ifeq ($(HOST_OS),darwin)
	@if [ -d "$(APP_PATH)" ]; then \
		bash scripts/codesign_bundle.sh "$(APP_PATH)"; \
	else \
		echo "sign-bundle: $(APP_PATH) not found"; \
		exit 1; \
	fi
else
	@echo "sign-bundle: not macOS, skipping code signing"
endif

build-darwin: ffmpeg-fetch yt-dlp-fetch
	@echo "Building macOS ($(HOST_ARCH)) app bundle..."
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh build/bin/videodl.app/Contents "$(HOST_PLATFORM)"
	@bash scripts/copy_yt_dlp_to_bundle.sh build/bin/videodl.app/Contents "$(HOST_PLATFORM)"
	@bash scripts/package_ffmpeg.sh build/bin/videodl.app/Contents/Resources/ffmpeg
	@bash scripts/codesign_bundle.sh build/bin/videodl.app

build-windows: ffmpeg-fetch yt-dlp-fetch
	@echo "Building Windows executable..."
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh build/bin windows-x64
	@bash scripts/copy_yt_dlp_to_bundle.sh build/bin windows-x64
	@bash scripts/package_ffmpeg.sh build/bin/resources/ffmpeg

build-linux: ffmpeg-fetch yt-dlp-fetch
	@echo "Building Linux executable..."
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh build/bin linux-x64
	@bash scripts/copy_yt_dlp_to_bundle.sh build/bin linux-x64
	@bash scripts/package_ffmpeg.sh build/bin/resources/ffmpeg

quick-build:
	wails build -clean
	@bash scripts/copy_ffmpeg_to_bundle.sh "$(BUNDLE_CONTENTS)" "$(BUNDLE_PLATFORM)"
	@bash scripts/copy_yt_dlp_to_bundle.sh "$(BUNDLE_CONTENTS)" "$(BUNDLE_PLATFORM)"
	@bash scripts/package_ffmpeg.sh "$(BUNDLE_FFMPEG)"
ifeq ($(HOST_OS),darwin)
	@bash scripts/codesign_bundle.sh "$(APP_PATH)"
endif
