.PHONY: build run clean test

BINARY := terminal-endpoint
CMD_DIR := ./cmd/terminal-endpoint

build:
	go build -o $(BINARY) $(CMD_DIR)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

tidy:
	go mod tidy

vet:
	go vet ./...

fmt:
	go fmt ./...
