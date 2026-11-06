BINARY := server

.PHONY: all vet test build clean

all: build

vet:
	go vet ./...

test: vet
	go test -race ./...

build: test
	go build -o $(BINARY) ./cmd/server

clean:
	rm -f $(BINARY)
