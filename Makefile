## build tooling

MAKEFLAGS += --silent

GOLANGCI_LINT_VERSION = v1.54.2

all: help

## help: Prints a list of available build targets.
help:
	echo "Usage: make <OPTIONS> ... <TARGETS>"
	echo ""
	echo "Available targets are:"
	echo ''
	sed -n 's/^##//p' ${PWD}/Makefile | column -t -s ':' | sed -e 's/^/ /'
	echo
	echo "Targets run by default are: `sed -n 's/^all: //p' ./Makefile | sed -e 's/ /, /g' | sed -e 's/\(.*\), /\1, and /'`"

## lint: Lint with golangci-lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}
	golangci-lint run --verbose --color always ./...

## fmt: Format with gofmt
fmt:
	go fmt ./...

# tidy: Tidy with go mod tidy
tidy:
	go mod tidy

## pre-commit: Chain lint + test
pre-commit: test lint

## test: Test with go test
test:
	go test -race -covermode=atomic -coverprofile=coverage.out ./... && go tool cover -html=coverage.out && rm coverage.out

## test-perf: Benchmark tests with go test -bench
test-perf:
	go test -benchmem -bench=. -coverprofile=coverage-bench.out ./... && go tool cover -html=coverage-bench.out && rm coverage-bench.out

.PHONY: lint fmt tidy pre-commit test test-perf
