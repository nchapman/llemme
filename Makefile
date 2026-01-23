.PHONY: build build-go build-web install test check clean release-%

# Build web UI (requires pnpm)
build-web:
	cd web && pnpm install && pnpm build

# Build Go binary only (fast iteration when web UI hasn't changed)
build-go:
	go build -o lleme .

# Full build with web UI
build: build-web build-go

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
	rm -f lleme
	go clean

# Release (usage: make release-patch, make release-minor, make release-major)
release-%:
	@./scripts/release.sh $*
