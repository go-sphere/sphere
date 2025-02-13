MODULE := $(shell go list -m)
MODULE_NAME := $(lastword $(subst /, ,$(MODULE)))
BUILD := $(shell git rev-parse --short HEAD)@$(shell date +%s)
CURRENT_OS := $(shell uname | tr "[A-Z]" "[a-z]")
CURRENT_ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

DOCKER_IMAGE ?= ghcr.io/tbxark/$(MODULE_NAME)
DOCKER_FILE := cmd/app/Dockerfile

LD_FLAGS := "-X $(MODULE)/internal/config.BuildVersion=$(BUILD)"
GO_BUILD := CGO_ENABLED=0 go build -trimpath -ldflags $(LD_FLAGS) -tags=jsoniter

.PHONY: init
init: ## Init all dependencies
	go mod download
	go get entgo.io/ent/cmd/ent@latest
	go get github.com/google/wire/cmd/wire@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/favadi/protoc-go-inject-tag@latest
	$(MAKE) install
	$(MAKE) gen-ent
	$(MAKE) gen-docs
	$(MAKE) gen-wire
	buf dep update
	go mod tidy

.PHONY: install
install: ## Install all dependencies
	go install github.com/TBXark/sphere/contrib/protoc-gen-sphere@latest
	go install github.com/TBXark/sphere/contrib/protoc-gen-route@latest
	go install github.com/TBXark/sphere/contrib/ent-gen-proto@latest

.PHONY: gen-proto
gen-proto: ## Generate proto files and run protoc plugins
	ent-gen-proto -path=./internal/pkg/database/schema
	buf generate
	protoc-go-inject-tag -input="./api/*/*/*.pb.go" -remove_tag_comment

.PHONY: gen-ent
gen-ent: ## Generate ent code
	go generate ./internal/pkg/database/generate.go

.PHONY: gen-docs
gen-docs: gen-proto ## Generate swagger docs
	go generate docs.go

.PHONY: gen-wire
gen-wire: ## Generate wire code
	go generate ./cmd/...

.PHONY: gen-conf
gen-conf: ## Generate example config
	go run ./cmd/cli/config gen

.PHONY: generate
generate: ## Run all generate command
	go generate ./...

.PHONY: dash
dash: ## Build dash
	sh ./assets/dash/build.sh

.PHONY: build
build: ## Build binary
	$(GO_BUILD) -o ./build/$(CURRENT_OS)_$(CURRENT_ARCH)/ ./...

.PHONY: build-linux-amd64
build-linux-amd64: ## Build linux amd64 binary
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_amd64/ ./...

.PHONY: build-linux-arm64
build-linux-arm64: ## Build linux arm64 binary
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o ./build/linux_arm64/ ./...

.PHONY: build-all
build-all: build-linux-amd64 build-linux-arm64 ## Build all arch binary

.PHONY: build-docker
build-docker: ## Build docker image
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) . -f  $(DOCKER_FILE) --push --provenance=false

.PHONY: delpoy
deploy: ## Deploy binary
	ansible-playbook -i devops/hosts/inventory.ini devops/delpoy-binary.yaml

.PHONY: lint
lint: ## Run linter
	golangci-lint run
	buf lint

.PHONY: fmt
fmt: ## Run formatter
	go fmt ./...
	buf format

.PHONY: clean
clean: ## Clean build files
	rm -rf ./build
	rm -rf ./swagger
	rm -rf ./api
	rm -rf ./internal/pkg/database/ent

.PHONY: help
help: ## Show this help message
	@echo "\n\033[1mSphere build tool.\033[0m Usage: make [target]\n"
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/\(.*\):.*##\(.*\)/\1:\2/' | column -t -s ':' |  sed -e 's/^/  /'