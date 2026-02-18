# List available recipes
default:
  @just --list

# Developer quick checks
ci: fmt test build

# Print Go toolchain version
doctor:
  go version

# Format Go code (gofmt)
fmt:
  gofmt -w .

# Run unit tests (forward args)
test *args:
  go test ./... {{args}}

# Build dist/oc
build:
  mkdir -p dist
  go build -o dist/oc ./cmd/oc

# Build then run oc (forward args)
run *args:
  just build
  ./dist/oc {{args}}

# Run oc against demo storage
demo:
  just build
  OC_STORAGE_ROOT="$PWD/demo/opencode-storage" OC_CONFIG_PATH="$PWD/demo/oc-config.yaml" ./dist/oc

# Remove build artifacts
clean:
  rm -rf dist
