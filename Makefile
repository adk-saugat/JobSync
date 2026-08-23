.PHONY: build run test tidy clean build-server release

BINARY := bin/jobsync
SERVER_BINARY := bin/jobsync-server
VERSION ?= dev
CLOUD_URL ?= https://jobsync-b7ltqpwroa-uc.a.run.app
OAUTH_JSON := internal/google/auth/oauth_client.json
AUTH_PKG := github.com/saugatadhikari/jobSync/internal/google/auth

LDFLAGS := -X github.com/saugatadhikari/jobSync/internal/cli.Version=$(VERSION)
LDFLAGS += -X github.com/saugatadhikari/jobSync/internal/config.DefaultCloudServerURL=$(CLOUD_URL)

# Bake Desktop OAuth into CLI binaries from local oauth_client.json (gitignored).
ifneq (,$(wildcard $(OAUTH_JSON)))
  OAUTH_CLIENT_ID := $(shell python3 -c 'import json; print(json.load(open("$(OAUTH_JSON)"))["installed"]["client_id"])')
  OAUTH_CLIENT_SECRET := $(shell python3 -c 'import json; print(json.load(open("$(OAUTH_JSON)"))["installed"]["client_secret"])')
  LDFLAGS += -X $(AUTH_PKG).EmbeddedClientID=$(OAUTH_CLIENT_ID)
  LDFLAGS += -X $(AUTH_PKG).EmbeddedClientSecret=$(OAUTH_CLIENT_SECRET)
endif

build:
	@test -f $(OAUTH_JSON) || (echo "Missing $(OAUTH_JSON) — copy oauth_client.json.example and fill in (see docs/DEPLOY.md)" >&2; exit 1)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/jobsync

build-server:
	go build -o $(SERVER_BINARY) ./cmd/server

release:
	@test -f $(OAUTH_JSON) || (echo "Missing $(OAUTH_JSON) — required to bake OAuth into release binaries" >&2; exit 1)
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/jobsync-darwin-arm64 ./cmd/jobsync
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/jobsync-darwin-amd64 ./cmd/jobsync
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/jobsync-linux-amd64 ./cmd/jobsync
	@echo "Binaries in dist/ (OAuth baked in from $(OAUTH_JSON))"

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/
