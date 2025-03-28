.PHONY: build-docker
build-docker: ## Build docker image
	docker build -t $(DOCKER_IMAGE) . -f $(DOCKER_FILE) --provenance=false --build-arg BUILD_VERSION=$(BUILD)

.PHONY: build-x-docker
build-x-docker: ## Build multi-arch docker image
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) . -f $(DOCKER_FILE) --push --provenance=false --build-arg BUILD_VERSION=$(BUILD)