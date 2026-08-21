.PHONY: build run test tidy clean

BINARY := bin/jobsync

build:
	go build -o $(BINARY) ./cmd/jobsync

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
