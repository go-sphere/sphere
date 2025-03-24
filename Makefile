.PHONY: install
install: ## Install all dependencies
	 cd contrib/protoc-gen-sphere && go mod tidy && go install .
	 cd contrib/protoc-gen-route && go mod tidy && go install .
	 cd contrib/ent-gen-proto && go mod tidy && go install .
	 cd contrib/sphere-cli && go mod tidy && go install .

.PHONY: lint
lint: ## Run linter
	golangci-lint run

.PHONY: format
format: ## Format code
	golangci-lint fmt
	golangci-lint run --fix
	go fmt ./...
	go mod tidy