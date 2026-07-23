BINARY := server

setup:
	git config core.hooksPath .githooks

vet:
	go vet ./...

test: vet
	go test -race ./...

build: test
	go build -o $(BINARY) ./cmd/server

clean:
	rm -f $(BINARY)

.PHONY: setup vet test build clean
