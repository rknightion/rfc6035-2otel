.PHONY: build test vet lint tidy tidy-check check fmt docker

build:
	go build -o bin/rfc6035-2otel ./cmd/rfc6035-2otel

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

tidy-check:
	go mod tidy -diff

docker:
	docker build -t rfc6035-2otel:dev .

check: vet test tidy-check
