# ---------- Makefile for Sphere Project ----------
MODULE         := $(shell go list -m)
MODULE_NAME    ?= $(lastword $(subst /, ,$(MODULE)))
# ---------- Build Config ----------
BUILD_TAG      ?= $(if $(BUILD_VERSION),$(BUILD_VERSION),$(shell git describe --tags --always --dirty 2>/dev/null || echo dev))
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
GO_TAGS	   	  ?= jsoniter#,embed_dash
GO_BUILD      ?= CGO_ENABLED=0 go build -trimpath -ldflags "$(LD_FLAGS)" -tags=$(GO_TAGS)
GO_RUN        ?= CGO_ENABLED=0 go run -ldflags "$(LD_FLAGS)" -tags=$(GO_TAGS)


# ---------- Build Tools ----------
.PHONY: build
build: ## Build binary
	$(GO_BUILD) -o ./build/current_arch/ ./...

.PHONY: build/linux/amd64
build/linux/amd64: ## Build linux amd64 binary
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_amd64/ ./...

.PHONY: build/linux/arm64
build/linux/arm64: ## Build linux arm64 binary
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o ./build/linux_arm64/ ./...

.PHONY: build/all
build/all: build build/linux/amd64 ## Build all arch binary


# ---------- Assets Tools ----------
.PHONY: build/assets
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

.PHONY: gen/dts
gen/dts: gen/dts ## Generate swagger typescript docs
	cd scripts/swagger-typescript-api-gen && npm run gen
ifneq ($(wildcard $(DASH_DIR)),)
	mkdir -p $(DASH_DIR)/src/api/swagger
	rm -rf $(DASH_DIR)/src/api/swagger/*
	cp -r swagger/dash/typescript/* $(DASH_DIR)/src/api/swagger
endif

# ---------- Generate Tools ----------
.PHONY: gen/proto
gen/proto: ## Generate proto files and run protoc plugins
	go tool sphere-cli ent2proto --path=./internal/pkg/database/schema
	go tool buf generate
	go tool protoc-go-inject-tag -input="./api/*/*/*.pb.go" -remove_tag_comment
	go run ./cmd/cli/gen-bind --file ./internal/pkg/render/bind.go --mod $(MODULE)

.PHONY: gen/db
gen/db: ## Generate ent code
	go generate ./internal/pkg/database/generate.go

.PHONY: gen/docs
gen/docs: gen/proto ## Generate swagger docs
	go generate docs.go

.PHONY: gen/wire
gen/wire: ## Generate wire code
	go generate ./cmd/...

.PHONY: gen/conf
gen/conf: ## Generate example config
	go run ./cmd/cli/config gen

.PHONY: clean
clean: ## Clean gen code and build files
	rm -rf ./build
	rm -rf ./swagger
	rm -rf ./api
	rm -rf ./internal/pkg/database/ent

.PHONY: gen/all
gen/all: clean ## Generate both ent, docs and wire
	$(MAKE) gen/db
	$(MAKE) gen/docs
	$(MAKE) gen/wire

# ---------- Build Docker ----------
.PHONY: build/docker
build/docker: ## Build docker image
	docker build -t $(DOCKER_IMAGE) . -f $(DOCKER_FILE) --provenance=false --build-arg BUILD_VERSION=$(BUILD_VER)

.PHONY: build/multi-docker
build/multi-docker: ## Build multi-arch docker image
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) . -f $(DOCKER_FILE) --push --provenance=false --build-arg BUILD_VERSION=$(BUILD_VER)

# ---------- Tools ----------
.PHONY: run
run: ## Run the application
	$(GO_RUN) $(MODULE)/cmd/app

.PHONY: delpoy
deploy: ## Deploy binary
	./devops/deploy/deploy.sh deploy

.PHONY: lint
lint: ## Run linter
	go tool golangci-lint run
	go tool buf lint

.PHONY: fmt
fmt: ## Run formatter
	go tool golangci-lint fmt
	go tool golangci-lint run --fix
	go mod tidy
	go fmt ./...
	go tool buf format

# ---------- Install Tools ----------
.PHONY: install
install: ## Install dependencies tools
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
ifeq ($(IGNORE_INSTALL_SPHERE_TOOLS),1)
	@echo "Skipping sphere tools installation as IGNORE_INSTALL_SPHERE_TOOLS=1"
else
	go install github.com/TBXark/sphere/contrib/protoc-gen-sphere@latest
	go install github.com/TBXark/sphere/contrib/protoc-gen-route@latest
endif

.PHONY: init
init: ## Init all dependencies
	go mod download
	$(MAKE) install
	$(MAKE) gen/all
	go tool buf dep update
	go mod tidy

.PHONY: help
help: ## Show this help message
	@echo "\n\033[1mSphere build tool.\033[0m Usage: make [target]\n"
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/\(.*\):.*##\(.*\)/\1:\2/' | column -t -s ':' |  sed -e 's/^/  /'

.DEFAULT_GOAL := help