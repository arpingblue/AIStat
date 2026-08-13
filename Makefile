BINARY := aistat
PKG := ./...

.PHONY: build test vet fmt staticcheck cross clean

build:
	CGO_ENABLED=0 go build -trimpath -o bin/$(BINARY) ./cmd/aistat

test:
	go test -race $(PKG)

vet:
	go vet $(PKG)

fmt:
	test -z "$$(gofmt -l cmd internal)"

staticcheck:
	staticcheck $(PKG)

cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/aistat-linux-amd64 ./cmd/aistat
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o dist/aistat-linux-arm64 ./cmd/aistat

clean:
	rm -rf -- bin dist coverage.out
