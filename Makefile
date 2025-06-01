.PHONY: install
install: ## Install all dependencies
	 cd contrib/protoc-gen-sphere && go mod tidy && go install .
	 cd contrib/protoc-gen-route && go mod tidy && go install .
	 #cd contrib/ent-gen-proto && go mod tidy && go install .
	 #cd contrib/sphere-cli && go mod tidy && go install .

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
	cd contrib/ent-gen-proto && go mod tidy
	cd contrib/ent-template-gen && go mod tidy
	cd contrib/protoc-gen-route && go mod tidy
	cd contrib/protoc-gen-sphere && go mod tidy
	cd contrib/README.md && go mod tidy
	cd contrib/sphere-cli && go mod tidy