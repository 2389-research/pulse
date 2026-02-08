.PHONY: build test test-race test-coverage install clean lint fmt run-mcp setup dev

.DEFAULT_GOAL := build

build:
	go build -o pulse ./cmd/pulse

test:
	go test -v ./...

test-race:
	go test -race ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

install:
	go install ./cmd/pulse

clean:
	rm -f pulse coverage.out coverage.html
	go clean

lint:
	golangci-lint run --timeout=10m

fmt:
	go fmt ./...

run-mcp: build
	./pulse mcp

setup: build
	./pulse setup

dev: fmt lint test build
