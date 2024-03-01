.PHONY: test test_cov

all:
	@echo "See Makefile for available commands"

build:
	go build

test:
	go test -v ./... -fullpath

test_fast:
	go test -v ./... -failfast -fullpath

test_bench:
	go test -v ./... -bench=. -fullpath

test_cov:
	go test -v ./... -coverprofile=.coverage -fullpath

test_cov_show:
	go tool cover -html=.coverage

# Usage:
# make publish ARGS=v0.2.0
#
# See https://stackoverflow.com/a/2214593/9788634
publish:
	git tag $(ARGS) && git push origin $(ARGS)
