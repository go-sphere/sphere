GOLANG_CI_LINT = golangci-lint
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

.PHONY: install
install: ## Install all dependencies
	 cd cmd/protoc-gen-sphere-binding && go mod tidy && go install .
	 cd cmd/protoc-gen-sphere-errors && go mod tidy && go install .
	 cd cmd/protoc-gen-sphere && go mod tidy && go install .
	 cd cmd/protoc-gen-route && go mod tidy && go install .
	 cd cmd/sphere-cli && go mod tidy && go install .

define fmt_mod
	cd $1 && go fmt ./... && $(GOLANG_CI_LINT) fmt && $(GOLANG_CI_LINT) run --fix && cd -
endef

define upgrade_mod
	cd $1 && go get -u ./... && go mod tidy && cd -
endef

define test_mod
	cd $1 && go test -v ./... && cd -
endef

.PHONY: fmt
fmt: ## Format code
	$(foreach mod,$(MODULES),$(call fmt_mod,$(mod)) && ) true

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	$(foreach mod,$(MODULES),$(call upgrade_mod,$(mod)) && ) true

.PHONY: test
test: ## Run tests
	$(foreach mod,$(MODULES),$(call test_mod,$(mod)) && ) true

.PHONY: cli/service/test
cli/service/test: ## Test sphere-cli service generation
	sphere-cli service golang --name KeyValueStore &> layout/internal/service/dash/keyvaluestore.go
	sphere-cli service proto --name KeyValueStore &> layout/proto/dash/v1/keyvaluestore.proto
	cd layout && make gen/all && make build

.PHONY: hook/before/commit
hook/before/commit: install fmt cli/service/test ## Run before commit hook
	cd layout && IGNORE_INSTALL_SPHERE_TOOLS=1 make install && make build