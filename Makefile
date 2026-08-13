BINARY := aistat
PKG := ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/arpingblue/AIStat/internal/version.Version=$(VERSION) -X github.com/arpingblue/AIStat/internal/version.Commit=$(COMMIT) -X github.com/arpingblue/AIStat/internal/version.Date=$(BUILD_DATE)

.PHONY: build test fuzz vet fmt staticcheck cross clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/aistat

test:
	go test -race $(PKG)

fuzz:
	go test ./internal/collector/cpu -run=^$$ -fuzz=^FuzzParseCPUList$$ -fuzztime=10s
	go test ./internal/collector/pci -run=^$$ -fuzz=^FuzzParseACS$$ -fuzztime=10s
	go test ./internal/collector/nvidia -run=^$$ -fuzz=^FuzzParseNvidiaCSV$$ -fuzztime=10s
	go test ./internal/collector/nvidia -run=^$$ -fuzz=^FuzzParseNvidiaTopology$$ -fuzztime=10s
	go test ./internal/collector/process -run=^$$ -fuzz=^FuzzParseAllowedArgs$$ -fuzztime=10s

vet:
	go vet $(PKG)

fmt:
	test -z "$$(gofmt -l cmd internal)"

staticcheck:
	staticcheck $(PKG)

cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/aistat-linux-amd64 ./cmd/aistat
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/aistat-linux-arm64 ./cmd/aistat

clean:
	rm -rf -- bin dist coverage.out
