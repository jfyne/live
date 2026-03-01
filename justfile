help:
    @ just -l

# Build web assets and update README
build:
    #!/bin/bash
    set -e
    if ! [ -d web/node_modules ]; then
        cd web && npm install
        cd -
    fi
    go generate web/build.go
    # Note: embedmd README update temporarily disabled until README is updated for v2
    # if ! command -v embedmd &> /dev/null
    # then
    #     GO111MODULE=off go get github.com/campoy/embedmd
    # fi
    # embedmd -w README.md

# Run all Go tests
test:
    go test -v -race ./...

# Run Go tests with coverage
test-coverage:
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run web/TypeScript tests
test-web:
    cd web && npm test

# Run all tests (Go + web)
test-all: test test-web

# Format Go code
fmt:
    go fmt ./...

# Run Go linter
vet:
    go vet ./...

# Run counter example
example-counter:
    cd examples/counter && go run main.go

# Build counter example binary
build-counter:
    cd examples/counter && go build -o counter main.go

# Clean build artifacts
clean:
    rm -rf web/node_modules
    rm -rf web/browser
    rm -f coverage.out coverage.html
    rm -f examples/counter/counter
