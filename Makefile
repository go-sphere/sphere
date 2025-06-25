.PHONY: install
install: ## Install all dependencies
	 cd cmd/protoc-gen-sphere && go mod tidy && go install .
	 cd cmd/protoc-gen-route && go mod tidy && go install .
	 cd cmd/sphere-cli && go mod tidy && go install .

.PHONY: lint
lint: ## Run linter
	go tool golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go tool golangci-lint fmt
	go tool golangci-lint run --fix
	go fmt ./...
	go mod tidy
	cd layout && go mod tidy
	cd cmd/protoc-gen-route && go mod tidy
	cd cmd/protoc-gen-sphere && go mod tidy
	cd cmd/sphere-cli && go mod tidy