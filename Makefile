.PHONY: build test

build:
	go build ./...
	go run . build

test:
	go test ./...
