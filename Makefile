MODULE := $(shell go list -m)

.PHONY: verify
verify:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || \
		{ echo "Go files need formatting:"; gofmt -l $$(find . -name '*.go' -not -path './vendor/*'); exit 1; }
	go mod tidy -diff
	go vet ./...
	go test ./...
	go test -race ./...

.PHONY: api-compat
api-compat:
	./scripts/check-api-compat.sh

.PHONY: lint
lint:
	go fix ./...
	go fmt ./...
	go vet ./...
	go get ./...
	go test ./...
	go mod tidy
	golangci-lint fmt --no-config --enable gofmt,goimports
	golangci-lint run --no-config --fix
	# -exclude-test-files: nilaway merges nil flows through a constructor's return
	# summary across call sites, so a test that passes a literal nil to assert a
	# guard (log/backend_merge_test.go:157) is reported at unrelated dereferences
	# of the wrapper's result. Tests deliberately exercise nil inputs; keeping
	# them analyzed produces only this noise. Non-test code is still analyzed.
	nilaway -include-pkgs="$(MODULE)" -exclude-test-files ./...

add-tags:
	@if [ -z "$(TAG)" ]; then echo "TAG not set. Use TAG=v0.0.1 make tags-root"; exit 1; fi
	git tag -s ${TAG} -m "$(TAG)"
	git push origin --tags
	echo "GOPROXY=direct GONOSUMDB=github.com/go-sphere/sphere go get github.com/go-sphere/sphere@$(TAG)"

del-tags:
	@if [ -z "$(TAG)" ]; then echo "TAG not set. Use TAG=v0.0.1 make del-tags"; exit 1; fi
	git tag -d ${TAG} || true
