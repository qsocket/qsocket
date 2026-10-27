# Build rules of the qsock binary.
#
# Every port of the installed Go toolchain is built from the same source tree,
# the binary of a target lands in build/<os>/qsock-<arch>. The target list is
# the one the toolchain reports, so `make targets` is the single source of
# truth for the build matrix of the CI as well.

BIN         = qsock
MODULE      = github.com/qsocket/qsocket
BUILD_DIR   = build
RELEASE_DIR = release
GO          = go

# EXE is the extension the host port gives its binaries, `.exe` on windows and
# empty everywhere else. The toolchain reports it, so a host it learns about
# later needs no change here.
EXE := $(shell $(GO) env GOEXE)

# VERSION is stamped into the binary, a checkout without tags falls back to
# the commit it was made from.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "?")

# The binaries are static and carry no debug information, the build paths and
# the VCS stamp are left out to keep them reproducible.
BUILD_FLAGS = -trimpath -buildvcs=false \
	-ldflags="-extldflags=-static -s -w -X $(MODULE)/config.Version=$(VERSION)"

# goos, goarch and outfile destructure an <os>/<arch> target name.
goos    = $(firstword $(subst /, ,$(1)))
goarch  = $(lastword $(subst /, ,$(1)))
outfile = $(BUILD_DIR)/$(call goos,$(1))/$(BIN)-$(call goarch,$(1))$(if $(filter windows/%,$(1)),.exe)

# Ports the toolchain lists but this source tree cannot build: ios and the 32
# bit android ports need an external (cgo) linker, plan9 and the wasm ports
# carry none of the terminal ioctls the pty package needs. The android ports
# left out here are built by the android-ndk goal.
SKIP_TARGETS = ios/amd64 ios/arm64 js/wasm wasip1/wasm \
	plan9/386 plan9/amd64 plan9/arm \
	android/386 android/amd64 android/arm

# TARGETS are the <os>/<arch> pairs which build without cgo, a new port of
# the toolchain is picked up as soon as it is listed by it.
TARGETS   := $(filter-out $(SKIP_TARGETS),$(shell $(GO) tool dist list))
PLATFORMS := $(sort $(foreach t,$(TARGETS),$(call goos,$(t))))

.PHONY: default
default:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BIN)$(EXE) .

# all builds every target of the toolchain, `make -j all` builds them in
# parallel since the targets share nothing but the source tree.
.PHONY: all
all: $(TARGETS)

# A goal of its own for every target, `make linux/arm64` builds a single one
# and is the entry point used by the build matrix of the CI.
.PHONY: $(TARGETS)
$(TARGETS):
	@mkdir -p $(BUILD_DIR)/$(call goos,$@)
	CGO_ENABLED=0 GOOS=$(call goos,$@) GOARCH=$(call goarch,$@) \
		$(GO) build $(BUILD_FLAGS) -o $(call outfile,$@) .

# A goal for every operating system, `make linux` builds all of its ports.
.PHONY: $(PLATFORMS)
$(PLATFORMS):
	@$(MAKE) --no-print-directory $(filter $@/%,$(TARGETS))

# targets prints the target list, one per line.
.PHONY: targets
targets:
	@printf '%s\n' $(TARGETS)

.PHONY: test
test:
	$(GO) vet ./...
	$(GO) test ./...

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(RELEASE_DIR)

# =============================================================================
# The goals below need a toolchain of their own, they are not part of `all`.
# =============================================================================

ANDROID_API   = 33
NDK_BIN       = $(ANDROID_NDK)/toolchains/llvm/prebuilt/linux-x86_64/bin
# A cgo build links against the libc of the platform, so it cannot be static.
CGO_BUILD_FLAGS = -trimpath -buildvcs=false \
	-ldflags="-s -w -X $(MODULE)/config.Version=$(VERSION)"

# android-ndk builds the android ports which need an external linker, the
# ANDROID_NDK variable has to point at a native development kit.
.PHONY: android-ndk
android-ndk:
	@test -n "$(ANDROID_NDK)" || { echo "ANDROID_NDK is not set!"; exit 1; }
	@mkdir -p $(BUILD_DIR)/android
	CC="$(NDK_BIN)/x86_64-linux-android$(ANDROID_API)-clang" GOOS=android GOARCH=amd64 \
		CGO_ENABLED=1 $(GO) build $(CGO_BUILD_FLAGS) -o $(BUILD_DIR)/android/$(BIN)-amd64 .
	CC="$(NDK_BIN)/i686-linux-android$(ANDROID_API)-clang" GOOS=android GOARCH=386 \
		CGO_ENABLED=1 $(GO) build $(CGO_BUILD_FLAGS) -o $(BUILD_DIR)/android/$(BIN)-386 .
	CC="$(NDK_BIN)/armv7a-linux-androideabi$(ANDROID_API)-clang" GOOS=android GOARCH=arm \
		CGO_ENABLED=1 $(GO) build $(CGO_BUILD_FLAGS) -o $(BUILD_DIR)/android/$(BIN)-arm .
	CC="$(NDK_BIN)/aarch64-linux-android$(ANDROID_API)-clang" GOOS=android GOARCH=arm64 \
		CGO_ENABLED=1 $(GO) build $(CGO_BUILD_FLAGS) -o $(BUILD_DIR)/android/$(BIN)-arm64 .

.PHONY: android-apk
android-apk:
	@mkdir -p $(BUILD_DIR)/android
	gomobile build -target android -androidapi $(ANDROID_API) -o $(BUILD_DIR)/android/$(BIN).apk

# The ios ports need a cgo compiler wrapping the xcode toolchain.
.PHONY: ios
ios:
	@mkdir -p $(BUILD_DIR)/ios
	GOOS=ios GOARCH=amd64 CGO_ENABLED=1 CC=$(CURDIR)/clangwrap.sh \
		$(GO) build $(CGO_BUILD_FLAGS) -o $(BUILD_DIR)/ios/$(BIN)-amd64 .
	GOOS=ios GOARCH=arm64 CGO_ENABLED=1 CC=$(CURDIR)/clangwrap.sh \
		$(GO) build $(CGO_BUILD_FLAGS) -o $(BUILD_DIR)/ios/$(BIN)-arm64 .
