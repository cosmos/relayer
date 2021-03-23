VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
SDKCOMMIT := $(shell go list -m -u -f '{{.Version}}' github.com/cosmos/cosmos-sdk)
GAIA_VERSION := v4.1.0
AKASH_VERSION := v0.10.2
WASMD_VERSION := v0.14.1

GOPATH := $(shell go env GOPATH)
GOBIN := $(GOPATH)/bin

all: ci-lint install

###############################################################################
# Build / Install
###############################################################################

LD_FLAGS = -X github.com/cosmos/relayer/cmd.Version=$(VERSION) \
	-X github.com/cosmos/relayer/cmd.Commit=$(COMMIT) \
	-X github.com/cosmos/relayer/cmd.SDKCommit=$(SDKCOMMIT) \
	-X github.com/cosmos/relayer/cmd.GaiaCommit=$(GAIACOMMIT)

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

# Compile the relayer as a shared library to be linked into another program
compile-clib:
	go build -v -mod=readonly -buildmode=c-shared -o librelayer.so ./clib

install: go.sum
	@echo "installing rly binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o $(GOBIN)/rly main.go

###############################################################################
# Tests / CI
###############################################################################
test:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/...

test-gaia:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestGaia*

test-akash:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestAkash*

coverage:
	@echo "viewing test coverage..."
	@go tool cover --html=coverage.out

lint:
	@golangci-lint run
	@find . -name '*.go' -type f -not -path "*.git*" | xargs gofmt -d -s
	@go mod verify

.PHONY: install build lint coverage clean

###############################################################################
# Chain Code Downloads
###############################################################################

get-gaia:
	@mkdir -p ./chain-code/
	@git clone --branch $(GAIA_VERSION) git@github.com:cosmos/gaia.git ./chain-code/gaia

build-gaia:
	@./scripts/build-gaia

build-akash:
	@./scripts/build-akash

get-akash:
	@mkdir -p ./chain-code/
	@git clone --branch $(AKASH_VERSION) git@github.com:ovrclk/akash.git ./chain-code/akash

get-chains: get-gaia get-akash get-wasmd

get-wasmd:
	@mkdir -p ./chain-code/
	@git clone --branch $(WASMD_VERSION) git@github.com:CosmWasm/wasmd.git ./chain-code/wasmd

build-wasmd:
	@./scripts/build-wasmd

delete-chains: 
	@echo "Removing the ./chain-code/ directory..."
	@rm -rf ./chain-code

check-swagger:
	which swagger || (GO111MODULE=off go get -u github.com/go-swagger/go-swagger/cmd/swagger)

update-swagger-docs: check-swagger
	swagger generate spec -o ./docs/swagger-ui/swagger.yaml
