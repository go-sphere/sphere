
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
	go mod download
	$(MAKE) generate
	$(MAKE) docs
	#$(MAKE) config

.PHONY: generate
generate:
	go generate ./...

.PHONY: config
config:
	go run ./cmd/config gen

.PHONY: docs
docs:
	rm -rf ./docs/dash
	rm -rf ./docs/api
	swag init --output ./docs/api  --exclude internal/server/dash --instanceName API  -g internal/server/api/web.go
	swag init --output ./docs/dash --exclude internal/server/api  --instanceName Dash -g internal/server/dash/web.go

.PHONY: typescript
typescript: docs
	npx swagger-typescript-api -p ./docs/api/API_swagger.json   -o ./docs/api/typescript  --modular
	npx swagger-typescript-api -p ./docs/dash/Dash_swagger.json -o ./docs/dash/typescript --modular

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

.PHONY: clean
clean:
	rm -rf ./build
	rm -rf ./docs/dash
	rm -rf ./docs/api

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  init          - Initialize the project"
	@echo "  generate      - Generate code"
	@echo "  config        - Generate config"
	@echo "  docs          - Generate API documentation"
	@echo "  typescript    - Generate TypeScript API"
	@echo "  dash          - Build dashboard"
	@echo "  build         - Build for current OS and architecture"
	@echo "  build-all     - Build for all supported platforms"
	@echo "  build-docker  - Build and push Docker image"
	@echo "  deploy        - Deploy using Ansible"
	@echo "  lint          - Run linter"
	@echo "  clean         - Clean build artifacts"
	@echo "  help          - Show this help message"
