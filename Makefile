all: install

mod:
	@go mod tidy

build: mod
	@go build -mod=readonly -o build/relayer main.go

install: mod
	@go build -mod=readonly -o ${GOBIN}/relayer main.go

.PHONY: all build install

# TODO: Port reproducable build scripts from gaia
# TODO: Build should output builds for macos|windows|linux
# TODO: make test should run ci-chains but all the way to an OPEN connection
#       and attempt to send a packet from ibc0 -> ibc1
# TODO: Add linting support
# TODO: add support for versioning
# TODO: add ldflags for version of sdk, gaia and relayer, other useful/important info
