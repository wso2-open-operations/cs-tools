BINARY := server

.PHONY: all setup vet test build clean

all: build

# Install the shared git hooks (run once after cloning).
setup:
	git -C "$(shell git rev-parse --show-toplevel)" config core.hooksPath .githooks

vet:
	go vet ./...

test: vet
	go test -race ./...

build: test
	go build -o $(BINARY) ./cmd/server

clean:
	rm -f $(BINARY)
