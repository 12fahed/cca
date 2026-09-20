BINARY := cca
CMD    := ./cmd/cca
DIST   := dist

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

# Static binaries are a hard requirement: the shipped artifact has no runtime
# dependencies and must run on hosts without a matching libc.
GOENV := CGO_ENABLED=0

PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: all build test test-race lint fmt build-all clean

all: build

build:
	$(GOENV) go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) $(CMD)

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	go vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi

fmt:
	gofmt -w .

build-all:
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out=$(DIST)/$(BINARY)-$$os-$$arch; \
		if [ "$$os" = windows ]; then out=$$out.exe; fi; \
		echo "building $$out"; \
		$(GOENV) GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags '$(LDFLAGS)' -o $$out $(CMD) || exit 1; \
	done

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
