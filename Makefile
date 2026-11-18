GO ?= go
GOLANGCI_LINT ?= golangci-lint

.PHONY: build fmt-check lint test e2e
build:
	$(GO) build -o bin/tkn-agent ./cmd/tkn-agent

fmt-check:
	test -z "$$(gofmt -l cmd internal test)"

lint:
	$(GOLANGCI_LINT) run ./...

test:
	$(GO) test -race -shuffle=on ./...

e2e: build
	$(GO) test -tags=e2e -count=1 -v ./test/e2e
