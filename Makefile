VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
all: ci-lint install

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
	@go test -mod=readonly -v -coverprofile coverage.out ./test/...

test-gaia:
	@go test -mod=readonly -v -coverprofile coverage.out ./test/... -tags gaia

coverage:
	@echo "viewing test coverage..."
	@go tool cover --html=coverage.out

ci-lint:
	@GO111MODULE=on golangci-lint run
	@find . -name '*.go' -type f -not -path "*.git*" | xargs gofmt -d -s
	@go mod verify

.PHONY: install build ci-lint coverage clean

# TODO: Port reproducable build scripts from gaia for relayer
# TODO: Full tested and working releases
