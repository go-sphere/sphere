.PHONY: install
install: ## Install all dependencies
	 cd contrib/protoc-gen-sphere && go mod download && go install .
	 cd contrib/protoc-gen-route && go mod download && go install .
	 cd contrib/ent-gen-proto && go mod download && go install .