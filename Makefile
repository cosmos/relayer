VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
all: ci-lint ci-test install

###############################################################################
# Build / Install
###############################################################################

LD_FLAGS = -X github.com/iqlusioninc/relayer/cmd.Version=$(VERSION) \
	-X github.com/iqlusioninc/relayer/cmd.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(LD_FLAGS)'

build: go.sum
ifeq ($(OS),Windows_NT)
	@echo "building rly binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/rly.exe main.go
else
	@echo "building rly binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/rly main.go
endif

build-zip: go.sum
	@echo "building rly binaries for windows, mac and linux"
	@GOOS=linux GOARCH=amd64 go build -mod=readonly $(BUILD_FLAGS) -o build/linux-amd64-rly main.go
	@GOOS=darwin GOARCH=amd64 go build -mod=readonly $(BUILD_FLAGS) -o build/darwin-amd64-rly main.go
	@GOOS=windows GOARCH=amd64 go build -mod=readonly $(BUILD_FLAGS) -o build/windows-amd64-rly.exe main.go
	@tar -czvf release.tar.gz ./build

install: go.sum
	@echo "installing rly binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o ${GOBIN}/rly main.go

###############################################################################
# Tests / CI
###############################################################################
test:
	@go test -v ./relayer/...

coverage:
	@echo "viewing test coverage..."
	@go tool cover --html=coverage.out

ci-test:
	@echo "executing unit tests..."
	@go test -mod=readonly -v -coverprofile coverage.out ./... 

ci-lint:
	@echo "running GolangCI-Lint..."
	@GO111MODULE=on golangci-lint run
	@echo "formatting..."
	@find . -name '*.go' -type f -not -path "*.git*" | xargs gofmt -d -s
	@echo "verifying modules..."
	@go mod verify

.PHONY: install build ci-test ci-lint coverage clean

# TODO: Port reproducable build scripts from gaia
# TODO: Build should output builds for macos|windows|linux
# TODO: make test should run ci-chains but all the way to an OPEN connection
#       and attempt to send a packet from ibc0 -> ibc1
# TODO: Add linting support
# TODO: add support for versioning
# TODO: add ldflags for version of sdk, gaia and relayer, other useful/important info
