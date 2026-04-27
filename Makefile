.PHONY := clean build test coverage tar
.DEFAULT_GOAL := build

clean:
	@rm -rf go-dedup coverage.out go-dedup.tar.bz2

build:
	@go build .
	@strip go-dedup

test: build
	@gotestsum -- -coverprofile=coverage.out

coverage:
	@go tool cover -html=coverage.out

tar:
	@find . -name '*.go' | tar -T- -cjf go-dedup.tar.bz2

