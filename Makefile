.PHONY: build run test tidy clean build-server release

BINARY := bin/jobsync
SERVER_BINARY := bin/jobsync-server
VERSION ?= dev
CLOUD_URL ?= https://jobsync-b7ltqpwroa-uc.a.run.app
LDFLAGS := -X github.com/saugatadhikari/jobSync/internal/cli.Version=$(VERSION) -X github.com/saugatadhikari/jobSync/internal/config.DefaultCloudServerURL=$(CLOUD_URL)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/jobsync

build-server:
	go build -o $(SERVER_BINARY) ./cmd/server

release:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/jobsync-darwin-arm64 ./cmd/jobsync
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/jobsync-darwin-amd64 ./cmd/jobsync
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/jobsync-linux-amd64 ./cmd/jobsync
	@echo "Binaries in dist/"

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
