GO ?= go
GOLANGCI_LINT ?= golangci-lint
NILAWAY ?= nilaway

DIRECT_DEPS_TEMPLATE := {{if and (not .Main) (not .Indirect) (not .Replace)}}{{.Path}}{{end}}

.DEFAULT_GOAL := check

.PHONY: deps-update tidy fmt test lint check verify api-compat add-tags del-tags

deps-update:
	@deps="$$(GOWORK=off $(GO) list -m -f '$(DIRECT_DEPS_TEMPLATE)' all)"; \
	if [ -n "$$deps" ]; then GOWORK=off $(GO) get -u $$deps; fi
	GOWORK=off $(GO) mod tidy

tidy:
	GOWORK=off $(GO) mod tidy

fmt:
	$(GO) fmt ./...
	$(GOLANGCI_LINT) fmt --no-config --enable gofmt --enable goimports

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) fmt --no-config --enable gofmt --enable goimports --diff
	$(GO) vet ./...
	$(GOLANGCI_LINT) run --no-config
	# Tests deliberately exercise nil inputs, which produces false positives
	# when nilaway merges constructor summaries across call sites.
	$(NILAWAY) -include-pkgs="$$($(GO) list -m)" -exclude-test-files ./...

check:
	GOWORK=off $(GO) mod tidy -diff
	$(MAKE) lint
	$(MAKE) test

verify: check
	$(GO) test -race ./...

api-compat:
	./scripts/check-api-compat.sh

add-tags:
	@test -n "$(TAG)" || { echo "TAG is required: make add-tags TAG=v0.0.1"; exit 1; }
	git tag -s $(TAG) -m "$(TAG)"
	git push origin --tags
	@echo "GOPROXY=direct GONOSUMDB=github.com/go-sphere/sphere go get github.com/go-sphere/sphere@$(TAG)"

del-tags:
	@test -n "$(TAG)" || { echo "TAG is required: make del-tags TAG=v0.0.1"; exit 1; }
	-git tag -d $(TAG)
