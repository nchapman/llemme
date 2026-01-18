.PHONY: build install test check clean release-%

# Build binary in current directory
build:
	go build -o llemme .

# Install to GOBIN (or GOPATH/bin)
install:
	go install .

# Run tests
test:
	go test ./...

# Format, vet, and test
check:
	go fmt ./...
	go vet ./...
	go test ./...

# Clean build artifacts
clean:
	rm -f llemme
	go clean

# Release (usage: make release-patch, make release-minor, make release-major)
release-%:
	@./scripts/release.sh $*
