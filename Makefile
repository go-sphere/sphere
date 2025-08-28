MODULE := $(shell go list -m)

.PHONY: lint
lint:
	go mod tidy
	go fmt ./...
	go test ./...
	golangci-lint fmt --no-config --enable gofmt,goimports
	golangci-lint run --no-config --fix
	nilaway -include-pkgs="$(MODULE)" ./...