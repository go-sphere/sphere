MODULE := $(shell go list -m)
MODULE_NAME := $(lastword $(subst /, ,$(MODULE)))
BUILD := $(shell git rev-parse --short HEAD)@$(shell date +%s)
CURRENT_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH := $(shell uname -m | tr '[:upper:]' '[:lower:]')

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
	$(MAKE) generate
	buf dep update
	go mod tidy

.PHONY: install
install: ## Install all dependencies
	cd contrib/protoc-gen-sphere && go mod download && go install .
	cd contrib/protoc-gen-route && go mod download && go install .
	cd contrib/ent-gen-proto && go mod download && go install .

.PHONY: gen-proto
gen-proto: ## Generate proto files and run protoc plugins
	ent-gen-proto -path=./internal/pkg/database/ent/schema
	buf generate
	protoc-go-inject-tag -input="./api/*/*/*.pb.go" -remove_tag_comment

.PHONY: gen-docs
gen-docs: gen-proto ## Generate swagger docs
	swag init --output ./swagger/api  --tags api.v1,shared.v1   --instanceName API  -g docs.go --parseDependency
	swag init --output ./swagger/dash --tags dash.v1,shared.v1  --instanceName Dash -g docs.go --parseDependency

.PHONY: gen-ts
gen-ts: gen-docs ## Generate typescript client
	npx swagger-typescript-api -p ./swagger/api/API_swagger.json   -o ./swagger/api/typescript  --modular --responses --extract-response-body --extract-response-error
	npx swagger-typescript-api -p ./swagger/dash/Dash_swagger.json -o ./swagger/dash/typescript --modular --responses --extract-response-body --extract-response-error

.PHONY: gen-ent
gen-ent: ## Generate ent code
	go generate ./internal/pkg/database/ent

.PHONY: gen-wire
gen-wire: ## Generate wire code
	go generate ./cmd/...

.PHONY: gen-conf
gen-conf: ## Generate example config
	go run ./cmd/cli/config gen

.PHONY: generate
generate: ## Run all generate command
	$(MAKE) gen-ent
	$(MAKE) gen-docs
	go generate ./...

.PHONY: dash
dash: ## Build dash
	sh ./assets/dash/build.sh

.PHONY: build
build: ## Build binary
	$(GO_BUILD) -o ./build/$(CURRENT_OS)_$(CURRENT_ARCH)/ ./...

.PHONY: build-linux-amd
build-linux-amd: ## Build linux amd64 binary
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_x86/ ./...

.PHONY: build-linux-arm
build-linux-arm: ## Build linux arm64 binary
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o ./build/linux_arm64/ ./...

.PHONY: build-all
build-all: build-linux-amd build-linux-arm ## Build all arch binary

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

.PHONY: help
help: ## Show this help message
	@echo "\n\033[1mSphere build tool.\033[0m Usage: make [target]\n"
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/\(.*\):.*##\(.*\)/\1:\2/' | column -t -s ':' |  sed -e 's/^/  /'