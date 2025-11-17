.PHONY: test
test: build
	@./test/bats/bin/bats test

.PHONY: build
build:
	@go build -o dist/job
