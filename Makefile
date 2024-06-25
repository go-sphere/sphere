BIN_NAME=openkol
BUILD=$(shell git rev-parse --short HEAD)@$(shell date +%s)
CURRENT_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH := $(shell uname -m | tr '[:upper:]' '[:lower:]')
LD_FLAGS="-X github.com/github.com/tbxark/go-base-api/config.BuildVersion=$(BUILD)"
GO_BUILD=CGO_ENABLED=0 go build -ldflags $(LD_FLAGS)

.PHONY: init
init:
	go get entgo.io/ent/cmd/ent@latest
	go get github.com/google/wire/cmd/wire@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go mod download
	make generate
	make docs
	make dash
	make config

.PHONY: generate
generate:
	go generate ./...

.PHONY: config
config:
	go run main.go config

.PHONY: docs
docs:
	rm -rf ./docs/dashboard
	rm -rf ./docs/api
	swag init --tags dashboard --output ./docs/dashboard --instanceName Dashboard
	swag init --tags api --output ./docs/api --instanceName API

.PHONY: run
run:
	go run -ldflags $(LD_FLAGS) main.go start

.PHONY: dash
dash:
	sh ./assets/build.sh

.PHONY: build
build:
	$(GO_BUILD) -o ./build/$(CURRENT_OS)_$(CURRENT_ARCH)/ ./...

.PHONY: buildLinuxX86
buildLinuxX86:
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_x86/ ./...

.PHONY: buildWindowsX86
buildWindowsX86:
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o ./build/windows_x86/ ./...

.PHONY: deploy
deploy: buildLinuxX86
	@echo Build Version: $(BUILD)
	./init/deploy.sh

.PHONY: deploy-dev
deploy-dev: buildLinuxX86
	@echo Build Version: $(BUILD)
	./init/deploy-dev.sh

.PHONY: buildAll
buildAll: buildLinuxX86 buildWindowsX86 build