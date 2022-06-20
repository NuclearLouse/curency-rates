VERSION=1.0.0
LDFLAGS=-ldflags "-X currency-rates/internal/service.version=${VERSION}"

.PHONY: build
build:
	go build ${LDFLAGS} -mod vendor -v ./main/currency-rates

.DEFAULT_GOAL := build