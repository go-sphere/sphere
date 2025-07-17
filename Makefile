GOLANG_CI_LINT = golangci-lint

.PHONY: install
install: ## Install all dependencies
	 cd cmd/protoc-gen-sphere-binding && go mod tidy && go install .
	 cd cmd/protoc-gen-sphere-errors && go mod tidy && go install .
	 cd cmd/protoc-gen-sphere && go mod tidy && go install .
	 cd cmd/protoc-gen-route && go mod tidy && go install .
	 cd cmd/sphere-cli && go mod tidy && go install .

define fmt_mod
	cd $1 && go get -u ./... && go mod tidy && go fmt ./... && go test ./... && $(GOLANG_CI_LINT) fmt && $(GOLANG_CI_LINT) run --fix
endef

.PHONY: fmt
fmt: ## Format code
	$(call fmt_mod,.)
	$(call fmt_mod,layout)
	$(call fmt_mod,cmd/protoc-gen-route)
	$(call fmt_mod,cmd/protoc-gen-sphere)
	$(call fmt_mod,cmd/protoc-gen-sphere-binding)
	$(call fmt_mod,cmd/protoc-gen-sphere-errors)
	$(call fmt_mod,cmd/sphere-cli)
	$(call fmt_mod,internal/tags)
	$(call fmt_mod,proto/errors)
	$(call fmt_mod,proto/options)

.PHONY: cli/service/test
cli/service/test: ## Test sphere-cli service generation
	sphere-cli service golang --name KeyValueStore &> layout/internal/service/dash/keyvaluestore.go
	sphere-cli service proto --name KeyValueStore &> layout/proto/dash/v1/keyvaluestore.proto
	cd layout && make gen/all && make build

.PHONY: hook/before/commit
hook/before/commit: install fmt cli/service/test ## Run before commit hook
	cd layout && IGNORE_INSTALL_SPHERE_TOOLS=1 make install && make build