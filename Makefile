BINARY  := flow
PKG     := ./cmd/flow
DIST    := dist
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BINDIR  := $(HOME)/.local/bin

.PHONY: all build install test vet tidy dist clean

all: vet test build

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) $(PKG)

# Link em ~/.local/bin: cada `make build` novo já atualiza o comando.
install: build
	@mkdir -p $(BINDIR)
	ln -sf $(CURDIR)/$(BINARY) $(BINDIR)/$(BINARY)
	@$(BINDIR)/$(BINARY) --version

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# Build cruzado para distribuição manual via GitHub Releases.
dist: clean
	@mkdir -p $(DIST)
	GOOS=windows GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-windows-amd64.exe $(PKG)
	GOOS=darwin  GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-darwin-arm64     $(PKG)
	GOOS=darwin  GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-darwin-amd64     $(PKG)
	GOOS=linux   GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-amd64      $(PKG)
	@echo "Binários em $(DIST)/ (versão $(VERSION))"

clean:
	@rm -rf $(DIST) $(BINARY) $(BINARY).exe
