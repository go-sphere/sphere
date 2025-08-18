SPHERE_CLI      ?= sphere-cli
NILAWAY_CLI     ?= nilaway
GOLANG_CI_LINT  ?= golangci-lint

MODULE := $(shell go list -m)

MODULES := . \
	layout \
	cmd/protoc-gen-route \
	cmd/protoc-gen-sphere \
	cmd/protoc-gen-sphere-binding \
	cmd/protoc-gen-sphere-errors \
	cmd/sphere-cli \
	proto/binding \
	proto/errors \
	proto/options

COMMANDS := cmd/protoc-gen-route \
			cmd/protoc-gen-sphere \
			cmd/protoc-gen-sphere-binding \
			cmd/protoc-gen-sphere-errors \
			cmd/sphere-cli

define install_mod
	echo "install $1" && ( \
		cd $1 && \
		go mod tidy && \
		go install ./... \
	)
endef

define fmt_mod
	echo "fmt $1" && ( \
		cd $1 && \
		go mod tidy && \
		go fmt ./... && \
		$(GOLANG_CI_LINT) fmt --no-config --enable gofmt,goimports && \
		$(GOLANG_CI_LINT) run --no-config --fix \
	)
endef

define upgrade_mod
	echo "upgrade $1" && ( \
		cd $1 && \
		go get -u ./... && \
		go mod tidy \
	)
endef

define test_mod
	echo "test $1" && ( \
		cd $1 && \
		go test -v ./... \
	)
endef

define nil_check
	echo "nilaway check $1" && ( \
		cd $1 && \
		$(NILAWAY_CLI) -include-pkgs="$(MODULE)" ./... \
	)
endef

.PHONY: install
install: ## Install all dependencies
	@$(foreach mod,$(COMMANDS),$(call install_mod,$(mod)) && ) true

.PHONY: fmt
fmt: ## Format code
	@$(foreach mod,$(MODULES),$(call fmt_mod,$(mod)) && ) true

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	@$(foreach mod,$(MODULES),$(call upgrade_mod,$(mod)) && ) true

.PHONY: test
test: ## Run tests
	@$(foreach mod,$(MODULES),$(call test_mod,$(mod)) && ) true

.PHONY: nilaway
nilaway: ## Run nilaway checks
	@$(foreach mod,$(MODULES),$(call nil_check,$(mod)) && ) true

.PHONY: cli/service/test
cli/service/test: ## Test sphere-cli service generation
	$(SPHERE_CLI) service golang --name KeyValueStore &> layout/internal/service/dash/keyvaluestore.go
	$(SPHERE_CLI) service proto --name KeyValueStore &> layout/proto/dash/v1/keyvaluestore.proto
	cd layout && make gen/all && make build

.PHONY: hook/before/commit
hook/before/commit: install fmt cli/service/test ## Run before commit hook
	cd layout && IGNORE_INSTALL_SPHERE_TOOLS=1 make install && make build

