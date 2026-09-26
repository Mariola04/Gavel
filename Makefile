# Tools installed with `go install` (golangci-lint, gosec) live here.
export PATH := $(shell go env GOPATH)/bin:$(PATH)

BIN := bin/gavel
GO_FILES = $(shell find . -name '*.go' -not -path './testdata/*')

.PHONY: fmt fmt-check vet lint test check run build

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$out" ]; then echo "Ficheiros por formatar:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test -race ./...

check: fmt-check vet lint test

run:
	go run ./cmd/gavel serve

build:
	go build -o $(BIN) ./cmd/gavel
