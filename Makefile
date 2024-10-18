
MODULE := $(shell go list -m)
MODULE_NAME := $(lastword $(subst /, ,$(MODULE)))
BUILD := $(shell git rev-parse --short HEAD)@$(shell date +%s)
CURRENT_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH := $(shell uname -m | tr '[:upper:]' '[:lower:]')

DOCKER_IMAGE := ghcr.io/tbxark/$(MODULE_NAME)
DOCKER_FILE := cmd/app/Dockerfile

LD_FLAGS := "-X $(MODULE)/config.BuildVersion=$(BUILD)"
GO_BUILD := CGO_ENABLED=0 go build -ldflags $(LD_FLAGS)


.PHONY: init
init:
	go get entgo.io/ent/cmd/ent@latest
	go get github.com/google/wire/cmd/wire@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/tbxark/sphere/cmd/cli/protoc-gen-sphere@latest
	go install github.com/favadi/protoc-go-inject-tag@latest
	go mod download
	buf dep update
	$(MAKE) generate
	$(MAKE) docs

.PHONY: gen-proto
gen-proto:
	buf generate
	protoc-go-inject-tag -input="./api/*/*/*.pb.go" -remove_tag_comment

.PHONY: gen-docs
gen-docs: gen-proto
	swag init --output ./docs/api  --tags api.v1,shared.v1   --instanceName API  -g docs.go
	swag init --output ./docs/dash --tags dash.v1,shared.v1  --instanceName Dash -g docs.go

.PHONY: gen-ts
gen-ts: docs
	npx swagger-typescript-api -p ./docs/api/API_swagger.json   -o ./docs/api/typescript  --modular
	npx swagger-typescript-api -p ./docs/dash/Dash_swagger.json -o ./docs/dash/typescript --modular

.PHONY: generate
generate:
	go generate ./...

.PHONY: config
config:
	go run ./cmd/cli/config gen

.PHONY: dash
dash:
	sh ./assets/dash/build.sh

.PHONY: build
build:
	$(GO_BUILD) -o ./build/$(CURRENT_OS)_$(CURRENT_ARCH)/ ./...

.PHONY: build-linux-amd
build-linux-amd:
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_x86/ ./...

.PHONY: build-linux-arm
build-linux-arm:
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o ./build/linux_arm64/ ./...

.PHONY: build-all
build-all: build-linux-amd build-linux-arm

.PHONY: build-docker
build-docker:
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) . -f  $(DOCKER_FILE) --push --provenance=false

.PHONY: delpoy
deploy:
	ansible-playbook -i devops/hosts/inventory.ini devops/delpoy-binary.yaml

.PHONY: lint
lint:
	golangci-lint run
	buf lint

.PHONY: fmt
fmt:
	go fmt ./...
	buf format

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  init                Install all dependencies"
	@echo "  gen-proto           Generate proto files"
	@echo "  gen-docs            Generate swagger docs"
	@echo "  gen-ts              Generate typescript client"
	@echo "  generate            Generate code"
	@echo "  config              Generate config"
	@echo "  dash                Build dash"
	@echo "  build               Build binary"
	@echo "  build-linux-amd     Build linux amd64 binary"
	@echo "  build-linux-arm     Build linux arm64 binary"
	@echo "  build-all           Build all binary"
	@echo "  build-docker        Build docker image"
	@echo "  deploy              Deploy binary"
	@echo "  lint                Run linter"
	@echo "  fmt                 Run formatter"
	@echo "  help                Show this help message"
	@echo ""