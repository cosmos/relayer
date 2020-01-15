# TODO: add versioning
# TODO: add ldflags for version of sdk

all: install

mod:
	@go mod tidy

build: mod
	@go build -mod=readonly -o build/relayer main.go

install: mod
	@go build -mod=readonly -o ${GOBIN}/relayer main.go

.PHONY: all build install