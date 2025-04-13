MODULE         := $(shell go list -m)
MODULE_NAME    ?= $(lastword $(subst /, ,$(MODULE)))
BUILD_TAG      ?= $(if $(BUILD_VERSION),$(BUILD_VERSION),$(shell git describe --tags --always --dirty 2>/dev/null || echo dev))
BUILD_TIME	   ?= $(shell date +"%Y%m%d-%H%M%S")
BUILD_VER      ?= $(BUILD_TAG)@$(BUILD_TIME)

CURRENT_OS     ?= $(shell uname | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH   ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

DOCKER_IMAGE  ?= ghcr.io/tbxark/$(MODULE_NAME)
DOCKER_FILE   ?= cmd/app/Dockerfile

DASH_DIR      ?= ../sphere-dashboard
DASH_DIST     ?= assets/dash/dashboard/dist

LD_FLAGS      ?= -X $(MODULE)/internal/config.BuildVersion=$(BUILD_VER)
GO_TAGS	   	  ?= jsoniter#,embed_dash
GO_BUILD      ?= CGO_ENABLED=0 go build -trimpath -ldflags "$(LD_FLAGS)" -tags=$(GO_TAGS)
GO_RUN        ?= CGO_ENABLED=0 go run -ldflags "$(LD_FLAGS)" -tags=$(GO_TAGS)