BIN := dashburn
PKG := ./cmd/dashburn
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test race fmt vet run tidy check

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

race:
	go test -race ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

run:
	go run $(PKG)

tidy:
	go mod tidy

check: fmt vet race
