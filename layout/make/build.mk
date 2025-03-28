.PHONY: dash
dash: ## Build dash
ifneq ($(wildcard $(DASH_DIR)),)
	# You can `git clone https://github.com/pure-admin/vue-pure-admin.git $(DASH_DIR)` to get the dash project
	cd $(DASH_DIR) && pnpm build
	cp -r $(DASH_DIR)/dist/* $(DASH_DIST)
else
	@echo "Skipping dash build - DASH_DIR does not exist"
endif
.PHONY: build
build: ## Build binary
	$(GO_BUILD) -o ./build/current_arch/ ./...

.PHONY: build-linux-amd64
build-linux-amd64: ## Build linux amd64 binary
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o ./build/linux_amd64/ ./...

.PHONY: build-linux-arm64
build-linux-arm64: ## Build linux arm64 binary
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o ./build/linux_arm64/ ./...

.PHONY: build-all
build-all: build build-linux-amd64 ## Build all arch binary

.PHONY: delpoy
deploy: build-linux-amd64 ## Deploy binary
	ansible-playbook -i devops/ansible/hosts/inventory.ini devops/ansible/deploy.yml

.PHONY: lint
lint: ## Run linter
	golangci-lint run
	buf lint

.PHONY: fmt
fmt: ## Run formatter
	golangci-lint fmt
	golangci-lint run --fix
	go mod tidy
	go fmt ./...
	buf format