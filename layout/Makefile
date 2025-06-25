# ---------- Makefile for Sphere Project ----------
MODULE         := $(shell go list -m)
MODULE_NAME    ?= $(lastword $(subst /, ,$(MODULE)))
# ---------- Build Config ----------
GIT_TAG        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_TAG      ?= $(if $(BUILD_VERSION),$(BUILD_VERSION),$(GIT_TAG))
BUILD_TIME	   ?= $(shell date +"%Y%m%d-%H%M%S")
BUILD_VER      ?= $(BUILD_TAG)@$(BUILD_TIME)
# ---------- Current Arch  ----------
CURRENT_OS     ?= $(shell uname | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH   ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
# ---------- Docker Config ----------
DOCKER_IMAGE  ?= ghcr.io/tbxark/$(MODULE_NAME)
DOCKER_FILE   ?= cmd/app/Dockerfile
# ---------- Dashboard Config ----------
DASH_DIR      ?= ../sphere-dashboard
DASH_DIST     ?= assets/dash/dashboard/dist
# ---------- Go Build Config ----------
LD_FLAGS      ?= -X $(MODULE)/internal/config.BuildVersion=$(BUILD_VER)
GO            ?= go
GO_TAGS	   	  ?= jsoniter#,embed_dash
GO_RUN        ?= CGO_ENABLED=0 go run -ldflags "$(LD_FLAGS)" -tags=$(GO_TAGS)
GO_BUILD      ?= CGO_ENABLED=0 go build -trimpath -ldflags "$(LD_FLAGS)" -tags=$(GO_TAGS)
GO_INSTALL    ?= go install
# ---------- Go Tools ----------
BUF_CLI          ?= buf
SWAG_CLI		 ?= swag
SPHERE_CLI       ?= sphere-cli
GOLANG_CI_LINT   ?= golangci-lint
INTERNAL_TOOLS   ?= $(GO) run -tags spheretools

.PHONY: \
	build build/linux/amd64 build/linux/arm64 build/all clean\
	gen/wire gen/conf gen/db gen/proto gen/docs gen/all gen/dts\
	build/assets build/docker build/multi-docker \
	run run/swag deploy lint fmt \
	install init help

# ---------- Build Tools ----------
build: ## Build binary
	$(GO_BUILD) -o ./build/current_arch/ ./...

build/linux/amd64: ## Build linux amd64 binary
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_amd64/ ./...

build/linux/arm64: ## Build linux arm64 binary
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o ./build/linux_arm64/ ./...

build/all: build build/linux/amd64 ## Build all arch binary

# ---------- Generate Tools ----------
clean: ## Clean gen code and build files
	rm -rf ./api/*
	rm -rf ./build/*
	rm -rf ./swagger/*
	rm -rf ./internal/pkg/database/ent/*

gen/wire: ## Generate wire code
	$(GO) generate ./cmd/...

gen/conf: ## Generate example config
	$(INTERNAL_TOOLS) ./cmd/tools/config gen

gen/db: ## Generate ent code
	$(INTERNAL_TOOLS) ./cmd/tools/ent

gen/proto: gen/db ## Generate proto files and run protoc plugins
	$(SPHERE_CLI) entproto --path=./internal/pkg/database/schema
	$(BUF_CLI) generate
	$(SPHERE_CLI) retags --input="./api/*/*/*.pb.go" --remove_tag_comment
	$(INTERNAL_TOOLS) ./cmd/tools/bind

gen/docs: gen/proto ## Generate swagger docs
	$(SWAG_CLI) init \
		--output ./swagger/api \
		--tags api.v1,shared.v1 \
		--instanceName API \
		-g docs.go \
		--parseDependency
	$(SWAG_CLI) init \
		--output ./swagger/dash \
		--tags dash.v1,shared.v1 \
		--instanceName Dash \
		-g docs.go \
		--parseDependency

gen/all: clean ## Generate both ent, docs and wire
	$(MAKE) gen/docs
	$(MAKE) gen/wire

# ---------- Assets Tools ----------
gen/dts: gen/docs ## Generate swagger typescript docs
	cd scripts/swagger-typescript-api-gen && npm run gen
ifneq ($(wildcard $(DASH_DIR)),)
	mkdir -p $(DASH_DIR)/src/api/swagger
	rm -rf $(DASH_DIR)/src/api/swagger/*
	cp -r swagger/dash/typescript/* $(DASH_DIR)/src/api/swagger
endif

build/assets: ## Build assets
ifneq ($(wildcard $(DASH_DIR)),)
	# You can `git clone https://github.com/pure-admin/vue-pure-admin.git $(DASH_DIR)` to get the dash project
	mkdir -p $(DASH_DIST)
	cd $(DASH_DIR) && pnpm build
	rm -rf $(DASH_DIST)/*
	cp -r $(DASH_DIR)/dist/* $(DASH_DIST)
else
	@echo "Skipping dash build - DASH_DIR does not exist"
endif

# ---------- Build Docker ----------
build/docker: ## Build docker image
	docker build \
		-t $(DOCKER_IMAGE) \
		. \
		-f $(DOCKER_FILE) \
		--provenance=false \
		--build-arg \
		BUILD_VERSION=$(BUILD_VER)

build/multi-docker: ## Build multi-arch docker image
	docker buildx build \
		--platform=linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE) \
		. \
		-f $(DOCKER_FILE) \
		--push \
		--provenance=false \
		--build-arg BUILD_VERSION=$(BUILD_VER)

# ---------- Tools ----------
run: ## Run the application
	$(GO_RUN) $(MODULE)/cmd/app

run/swag: ## Run the swagger server
	$(INTERNAL_TOOLS) $(MODULE)/cmd/tools/docs

deploy: ## Deploy binary
	./devops/deploy/deploy.sh deploy

lint: ## Run linter
	$(GOLANG_CI_LINT) run
	$(BUF_CLI) lint

fmt: ## Run formatter
	$(GOLANG_CI_LINT) fmt
	$(GOLANG_CI_LINT) run --fix
	$(BUF_CLI) format
	$(GO) mod tidy
	$(GO) fmt ./...

# ---------- Install Tools ----------
install: ## Install dependencies tools
	$(GO_INSTALL) github.com/google/wire/cmd/wire@latest
	$(GO_INSTALL) github.com/swaggo/swag/cmd/swag@latest
	$(GO_INSTALL) github.com/bufbuild/buf/cmd/buf@latest
	$(GO_INSTALL) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO_INSTALL) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
ifeq ($(IGNORE_INSTALL_SPHERE_TOOLS),1)
	@echo "Skipping sphere tools installation as IGNORE_INSTALL_SPHERE_TOOLS=1"
else
	$(GO_INSTALL) github.com/TBXark/sphere/cmd/sphere-cli@latest
	$(GO_INSTALL) github.com/TBXark/sphere/cmd/protoc-gen-sphere@latest
	$(GO_INSTALL) github.com/TBXark/sphere/cmd/protoc-gen-route@latest
endif

init: ## Init all dependencies
	$(GO) mod download
	$(MAKE) install
	$(MAKE) gen/all
	$(BUF_CLI) dep update
	$(GO) mod tidy

help: ## Show this help message
	@echo "\n\033[1mSphere build tool.\033[0m Usage: make [target]\n"
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/\(.*\):.*##\(.*\)/\1:\2/' | column -t -s ':' |  sed -e 's/^/  /'

.DEFAULT_GOAL := help