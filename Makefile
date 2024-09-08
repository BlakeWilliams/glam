PKG ?= ./...
GO ?= go
GOFMT ?= gofmt "-s"
GOLANGCI_LINT ?= $(shell which golangci-lint)
SRC := $(shell find . -name '*.go')

all: test lint

.PHONY: deps
deps:
	go mod tidy

.PHONY: test
test:
	go test -race $(PKG) -cover -coverpkg=$(PKG)

.PHONY: lint
lint:
ifndef GOLANGCI_LINT
	$(error golangci-lint not installed)
endif
	golangci-lint run $(PKG)

cover:
	go test -coverpkg=$(PKG) -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out
	rm coverage.out

.PHONY: update-deps
update-deps:
	go get -u
	go mod tidy

.PHONY: fmt-check
fmt-check:
	@diff=$$($(GOFMT) -d $(SRC)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;
