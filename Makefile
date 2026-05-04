.PHONY: build test run clean fmt lint

BINARY=lchat

build:
	go build -o $(BINARY) .

test:
	go test ./... -v -count=1 -timeout 120s

test-short:
	go test ./... -count=1 -timeout 60s -short

run:
	go run .

fmt:
	gofmt -w .

lint:
	test -z $$(gofmt -l .)

clean:
	rm -f $(BINARY)
	rm -f $(BINARY).exe
