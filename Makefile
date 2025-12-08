VERSION ?= dev

.PHONY: test
test: build unit-test integration-test

.PHONY: unit-test
unit-test:
	@go test ./...

.PHONY: integration-test
integration-test: build
	@./test/bats/bin/bats test

.PHONY: build
build:
	@go build -ldflags "-X github.com/juanibiapina/gob/internal/version.Version=$(VERSION)" -o dist/gob
