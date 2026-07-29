APP := uc
VERSION ?= dev
BIN_DIR ?= bin
DIST_DIR ?= dist
INSTALL_DIR ?= $(HOME)/.local/bin
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet check install dist dist-darwin dist-linux dist-windows clean

all: build

## build: Compile uc for the current platform.
build:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP) .

## test: Run the unit tests.
test:
	go test ./...

## vet: Run Go's static analysis.
vet:
	go vet ./...

## check: Format-check, vet, and test the project.
check:
	@test -z "$$(gofmt -l *.go)" || (echo "Run gofmt on the files above" && exit 1)
	$(MAKE) vet
	$(MAKE) test

## install: Build and install a development binary in ~/.local/bin.
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BIN_DIR)/$(APP) $(INSTALL_DIR)/$(APP)
	@echo "Installed $(INSTALL_DIR)/$(APP)"

## dist: Build release archives and checksums for macOS, Linux, and Windows on AMD64 and ARM64.
dist: clean
	@mkdir -p $(DIST_DIR)
	$(MAKE) -j dist-darwin dist-linux dist-windows
	@cd $(DIST_DIR) && \
	if command -v sha256sum >/dev/null 2>&1; then \
		sha256sum *.tar.gz *.zip > uc_checksums.txt; \
	else \
		shasum -a 256 *.tar.gz *.zip > uc_checksums.txt; \
	fi

## dist-darwin: Build macOS release archives (amd64, arm64).
dist-darwin:
	@mkdir -p $(DIST_DIR)/darwin_amd64 $(DIST_DIR)/darwin_arm64
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/darwin_amd64/$(APP) .
	tar -C $(DIST_DIR)/darwin_amd64 -czf $(DIST_DIR)/$(APP)_darwin_amd64.tar.gz $(APP)
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/darwin_arm64/$(APP) .
	tar -C $(DIST_DIR)/darwin_arm64 -czf $(DIST_DIR)/$(APP)_darwin_arm64.tar.gz $(APP)
	@rm -rf $(DIST_DIR)/darwin_amd64 $(DIST_DIR)/darwin_arm64

## dist-linux: Build Linux release archives (amd64, arm64).
dist-linux:
	@mkdir -p $(DIST_DIR)/linux_amd64 $(DIST_DIR)/linux_arm64
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux_amd64/$(APP) .
	tar -C $(DIST_DIR)/linux_amd64 -czf $(DIST_DIR)/$(APP)_linux_amd64.tar.gz $(APP)
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux_arm64/$(APP) .
	tar -C $(DIST_DIR)/linux_arm64 -czf $(DIST_DIR)/$(APP)_linux_arm64.tar.gz $(APP)
	@rm -rf $(DIST_DIR)/linux_amd64 $(DIST_DIR)/linux_arm64

## dist-windows: Build Windows release archives (amd64, arm64).
dist-windows:
	@mkdir -p $(DIST_DIR)/windows_amd64 $(DIST_DIR)/windows_arm64
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/windows_amd64/$(APP).exe .
	zip -jq $(DIST_DIR)/$(APP)_windows_amd64.zip $(DIST_DIR)/windows_amd64/$(APP).exe
	GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/windows_arm64/$(APP).exe .
	zip -jq $(DIST_DIR)/$(APP)_windows_arm64.zip $(DIST_DIR)/windows_arm64/$(APP).exe
	@rm -rf $(DIST_DIR)/windows_amd64 $(DIST_DIR)/windows_arm64

## clean: Remove local build and release artifacts.
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
