VERSION ?= dev

.PHONY: test
test: build
	@./test/bats/bin/bats test

.PHONY: build
build:
	@go build -ldflags "-X github.com/juanibiapina/gob/cmd.version=$(VERSION)" -o dist/gob
