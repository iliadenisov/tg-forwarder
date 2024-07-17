.DEFAULT_GOAL := build

fmt:
  go fmt ./...
.PHONY:fmt

vet: fmt
  go vet ./...
.PHONY:vet

test:
  @mkdir -p artifacts/test
  @go clean -testcache
# first, run tests with stdout
  @go test -cover ./...
# second, run cached tests with reports output
  @go test ./... -coverprofile artifacts/test/coverage.out -json > artifacts/test/coverage.json
  @go tool cover -html artifacts/test/coverage.out -o artifacts/test/coverage.html

build: vet test
  go build -o artifacts/bin/forwarder cmd/forwarder/main.go
.PHONY:build
