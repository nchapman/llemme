.PHONY: build install test check clean

# Build binary in current directory
build:
	go build -o lemme .

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
	rm -f lemme
	go clean
