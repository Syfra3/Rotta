BINARY = uncle-bob
BUILD_DIR = bin
MODULE = github.com/Syfra3/uncle-bob-workflow

.PHONY: build run test tidy lint clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/uncle-bob

run:
	go run ./cmd/uncle-bob

test:
	go test ./...

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
