VERSION ?= dev
LDFLAGS  = -ldflags "-X ship/cmd.Version=$(VERSION)"
BINARY   = ship.exe

.PHONY: build test clean install

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@rm -f coverage.out

clean:
	rm -f $(BINARY) coverage.out

install: build
	cp $(BINARY) $(GOPATH)/bin/$(BINARY)
