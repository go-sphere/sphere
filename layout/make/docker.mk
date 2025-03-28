.PHONY: build-docker
build-docker: ## Build docker image
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) . -f $(DOCKER_FILE) --push --provenance=false