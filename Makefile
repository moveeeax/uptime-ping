BINARY := uptime-ping
PKG := ./...
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all build test race vet fmt fmtcheck lint smoke run clean

all: fmtcheck vet test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test $(PKG)

race:
	go test -race $(PKG)

vet:
	go vet $(PKG)

fmt:
	gofmt -w .

fmtcheck:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Build then validate the example config without touching the network.
smoke: build
	./$(BINARY) --version
	./$(BINARY) --config examples/checks.yaml --validate

run: build
	./$(BINARY) --config examples/checks.yaml

clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist
