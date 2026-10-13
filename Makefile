# Makefile for the illsample library and CLI.
#
# Common targets:
#   make build         Build the CLI binary into ./bin/illsample
#   make test          Run all tests with the race detector
#   make cover         Print a coverage summary
#   make cover-html    Write an HTML coverage report to coverage.html
#   make fuzz          Run a short fuzz pass on the parser
#   make vet           Run go vet
#   make fmt           Run gofmt and goimports
#   make clean         Remove build artifacts
#   make install       Install the CLI to $GOPATH/bin
#
# Variables:
#   GO                 Go binary to use (default: go)
#   BIN                Output directory for the binary (default: bin)

GO ?= go
BIN ?= bin
LDFLAGS ?=

.PHONY: all build test cover cover-html fuzz vet fmt clean install help

all: build

build:
	@mkdir -p $(BIN)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN)/illsample ./cmd/illsample

test:
	$(GO) test -race ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -20
	@rm -f coverage.out

cover-html:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report at coverage.html"

fuzz:
	$(GO) test -fuzz=FuzzParse -fuzztime=10s ./internal/parser/

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(BIN) coverage.out coverage.html

install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/illsample

help:
	@echo "Targets:"
	@echo "  build         Build the CLI binary"
	@echo "  test          Run all tests with -race"
	@echo "  cover         Print coverage summary"
	@echo "  cover-html    Write HTML coverage report"
	@echo "  fuzz          Run parser fuzz for 10s"
	@echo "  vet           Run go vet"
	@echo "  fmt           Run gofmt"
	@echo "  clean         Remove build artifacts"
	@echo "  install       Install CLI to \$$GOPATH/bin"
