MODULE         := $(shell go list -m)
MODULE_NAME    := $(lastword $(subst /, ,$(MODULE)))
BUILD          := $(if $(BUILD_VERSION),$(BUILD_VERSION),$(shell git rev-parse --short HEAD 2>/dev/null || echo dev)@$(shell date +%s))

CURRENT_OS     := $(shell uname | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH   := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

DOCKER_IMAGE  ?= ghcr.io/tbxark/$(MODULE_NAME)
DOCKER_FILE   := cmd/app/Dockerfile

DASH_DIR      := ../sphere-dashboard
DASH_DIST     := assets/dash/dashboard/dist

LD_FLAGS      := -X $(MODULE)/internal/config.BuildVersion=$(BUILD)
GO_BUILD      := CGO_ENABLED=0 go build -trimpath -ldflags "$(LD_FLAGS)" -tags=jsoniter #add -tags=embed_dash when you want to embed the dashboard in the binary