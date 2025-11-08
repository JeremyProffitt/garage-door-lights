.PHONY: build-DevicesFunction

build-DevicesFunction:
	@echo "Starting build for DevicesFunction..."
	@echo "Current directory: $$(pwd)"
	@echo "Files in directory:"
	@ls -la
	go mod tidy || (echo "go mod tidy failed" && exit 1)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly -tags lambda.norpc -o bootstrap main.go || (echo "go build failed" && exit 1)
	@echo "Build complete. Checking for bootstrap:"
	@ls -la bootstrap || echo "ERROR: bootstrap not found!"
