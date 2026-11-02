BINARY := server

setup:
	git config core.hooksPath .githooks

vet:
	go vet ./...

test: vet
	go test -race ./...

build: test
	go build -o $(BINARY) ./cmd/server

# Writes catalogue-parts/ from internal/catalogue/catalogue.md, the files
# mounted in Choreo. Run after editing catalogue.md and commit the result.
catalogue-parts:
	go run ./cmd/splitcatalogue

clean:
	rm -f $(BINARY)

.PHONY: setup vet test build catalogue-parts clean
