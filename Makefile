VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
GAIA_VERSION := v6.0.0
AKASH_VERSION := v0.12.1
OSMOSIS_VERSION := v6.4.0
WASMD_VERSION := v0.16.0

GOPATH := $(shell go env GOPATH)
GOBIN := $(GOPATH)/bin

all: lint install

###############################################################################
# Build / Install
###############################################################################

LD_FLAGS = -X github.com/cosmos/relayer/v2/cmd.Version=$(VERSION)

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
	@go build -mod=readonly $(BUILD_FLAGS) -o $(GOBIN)/rly main.go

build-gaia-docker:
	docker build -t cosmos/gaia:$(GAIA_VERSION) --build-arg VERSION=$(GAIA_VERSION) -f ./docker/gaiad/Dockerfile .

build-akash-docker:
	docker build -t ovrclk/akash:$(AKASH_VERSION) --build-arg VERSION=$(AKASH_VERSION) -f ./docker/akash/Dockerfile .

build-osmosis-docker:
	docker build -t osmosis-labs/osmosis:$(OSMOSIS_VERSION) --build-arg VERSION=$(OSMOSIS_VERSION) -f ./docker/osmosis/Dockerfile .

###############################################################################
# Tests / CI
###############################################################################

test:
	@go test -mod=readonly -race ./...

test-integration:
	@go test -mod=readonly -v -timeout 20m ./_test/

test-gaia:
	@go test -mod=readonly -v -run TestGaiaToGaiaRelaying ./_test/
	@go test -mod=readonly -v -run TestRelayAllChannelsOnConnection ./_test/

test-akash:
	@go test -mod=readonly -v -run TestAkashToGaiaRelaying ./_test/

test-short:
	@go test -mod=readonly -v -run TestOsmoToGaiaRelaying ./_test/

coverage:
	@echo "viewing test coverage..."
	@go tool cover --html=coverage.out

lint:
	@golangci-lint run
	@find . -name '*.go' -type f -not -path "*.git*" | xargs gofmt -d -s
	@go mod verify

###############################################################################
# Chain Code Downloads
###############################################################################

get-gaia:
	@mkdir -p ./chain-code/
	@git clone --branch $(GAIA_VERSION) --depth=1 https://github.com/cosmos/gaia.git ./chain-code/gaia

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

build-chains: build-akash build-gaia build-wasmd

delete-chains: 
	@echo "Removing the ./chain-code/ directory..."
	@rm -rf ./chain-code

.PHONY: two-chains test test-integration install build lint coverage clean
