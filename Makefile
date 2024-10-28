
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
	go mod download
	go get entgo.io/ent/cmd/ent@latest
	go get github.com/google/wire/cmd/wire@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/favadi/protoc-go-inject-tag@latest
	$(MAKE) install
	$(MAKE) generate
	buf dep update

.PHONY: install
install:
	cd contrib/protoc-gen-sphere &&  go mod download && go install .
	cd contrib/ent-gen-proto &&  go mod download && go install .

.PHONY: gen-proto
gen-proto:
	ent-gen-proto
	buf generate
	protoc-go-inject-tag -input="./api/*/*/*.pb.go" -remove_tag_comment

.PHONY: gen-docs
gen-docs: gen-proto
	swag init --output ./swagger/api  --tags api.v1,shared.v1   --instanceName API  -g docs.go
	swag init --output ./swagger/dash --tags dash.v1,shared.v1  --instanceName Dash -g docs.go

.PHONY: gen-ts
gen-ts: gen-docs
	npx swagger-typescript-api -p ./swagger/api/API_swagger.json   -o ./swagger/api/typescript  --modular --responses --extract-response-body --extract-response-error
	npx swagger-typescript-api -p ./swagger/dash/Dash_swagger.json -o ./swagger/dash/typescript --modular --responses --extract-response-body --extract-response-error

.PHONY: gen-ent
gen-ent:
	go generate ./internal/pkg/database/ent

.PHONY: gen-wire
gen-wire:
	go generate ./cmd/...

.PHONY: gen-conf
gen-conf:
	go run ./cmd/cli/config gen

.PHONY: generate
generate:
	go generate ./...
	$(MAKE) gen-docs

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
	@echo "  gen-ent             Generate ent code"
	@echo "  gen-wire            Generate wire code"
	@echo "  gen-conf            Generate config"
	@echo "  generate            Generate code"
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