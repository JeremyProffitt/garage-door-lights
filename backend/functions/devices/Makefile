.PHONY: build-DevicesFunction

build-DevicesFunction:
	@echo "Starting build for DevicesFunction..."
	@echo "Current directory: $$(pwd)"
	@echo "Artifacts directory: $(ARTIFACTS_DIR)"
	go mod tidy || (echo "go mod tidy failed" && exit 1)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly -tags lambda.norpc -o $(ARTIFACTS_DIR)/bootstrap main.go || (echo "go build failed" && exit 1)
	@echo "Build complete. Checking bootstrap in artifacts:"
	@ls -la $(ARTIFACTS_DIR)/bootstrap
