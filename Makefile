GOLANG_CI_LINT = golangci-lint

.PHONY: install
install: ## Install all dependencies
	 cd cmd/protoc-gen-sphere && go mod tidy && go install .
	 cd cmd/protoc-gen-route && go mod tidy && go install .
	 cd cmd/sphere-cli && go mod tidy && go install .

define fmt_mod
	cd $1 && go mod tidy && go fmt ./... && go test ./... && $(GOLANG_CI_LINT) fmt && $(GOLANG_CI_LINT) run --fix
endef

.PHONY: fmt
fmt: ## Format code
	$(call fmt_mod,.)
	$(call fmt_mod,layout)
	$(call fmt_mod,cmd/protoc-gen-route)
	$(call fmt_mod,cmd/protoc-gen-sphere)
	$(call fmt_mod,cmd/sphere-cli)
	$(call fmt_mod,internal/protogo)
	$(call fmt_mod,internal/tags)