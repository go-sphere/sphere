MODULE := $(shell go list -m)

.PHONY: lint
lint:
	go fmt ./...
	go vet ./...
	go get ./...
	go test ./...
	go mod tidy
	golangci-lint fmt --no-config --enable gofmt,goimports
	golangci-lint run --no-config --fix
	nilaway -include-pkgs="$(MODULE)" ./...

tags-root:
	@if [ -z "$(TAG)" ]; then echo "TAG not set. Use TAG=v0.0.1 make tags"; exit 1; fi
	git tag -s ${TAG} -m "$(TAG)"
	git push origin --tags
	echo "GOPROXY=direct GONOSUMDB=github.com/go-sphere/sphere go get github.com/go-sphere/sphere@$(TAG)"
