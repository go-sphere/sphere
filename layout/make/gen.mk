.PHONY: init
init: ## Init all dependencies
	go mod download
	$(MAKE) install
	$(MAKE) gen-ent
	$(MAKE) gen-docs
	$(MAKE) gen-wire
	buf dep update
	go mod tidy

.PHONY: install
install: ## Install dependencies tools
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
ifeq ($(IGNORE_INSTALL_SPHERE_TOOLS),1)
	@echo "Skipping sphere tools installation as IGNORE_INSTALL_SPHERE_TOOLS=1"
else
	go install github.com/TBXark/sphere/contrib/protoc-gen-sphere@latest
	go install github.com/TBXark/sphere/contrib/protoc-gen-route@latest
endif

.PHONY: gen-proto
gen-proto: ## Generate proto files and run protoc plugins
	go tool ent-gen-proto -path=./internal/pkg/database/schema
	go tool buf generate
	go tool protoc-go-inject-tag -input="./api/*/*/*.pb.go" -remove_tag_comment
	go run ./cmd/cli/gen-bind --file ./internal/pkg/render/bind.go --mod $(MODULE)

.PHONY: gen-ent
gen-ent: ## Generate ent code
	go generate ./internal/pkg/database/generate.go

.PHONY: gen-docs
gen-docs: gen-proto ## Generate swagger docs
	go generate docs.go

.PHONY: gen-ts-docs
gen-ts-docs: gen-docs ## Generate swagger typescript docs
	cd scripts/swagger-typescript-api-gen && npm run gen
ifneq ($(wildcard $(DASH_DIR)),)
	mkdir -p $(DASH_DIR)/src/api/swagger
	rm -rf $(DASH_DIR)/src/api/swagger/*
	cp -r swagger/dash/typescript/* $(DASH_DIR)/src/api/swagger
endif

.PHONY: gen-wire
gen-wire: ## Generate wire code
	go generate ./cmd/...

.PHONY: gen-conf
gen-conf: ## Generate example config
	go run ./cmd/cli/config gen

.PHONY: gen-all
gen-all: clean ## Generate both ent, docs and wire
	$(MAKE) gen-ent
	$(MAKE) gen-docs
	$(MAKE) gen-wire

.PHONY: clean
clean: ## Clean gen code and build files
	rm -rf ./build
	rm -rf ./swagger
	rm -rf ./api
	rm -rf ./internal/pkg/database/ent