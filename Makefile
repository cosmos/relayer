VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
GAIA_VERSION := v7.0.1
AKASH_VERSION := v0.16.3
OSMOSIS_VERSION := v8.0.0
WASMD_VERSION := v0.25.0

GOPATH := $(shell go env GOPATH)
GOBIN := $(GOPATH)/bin

all: lint install

###############################################################################
# Build / Install
###############################################################################

ldflags = -X github.com/cosmos/relayer/v2/cmd.Version=$(VERSION)

ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -ldflags '$(ldflags)'

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

ibctest:
	cd ibctest && go test -race -v -run TestRelayerInProcess .

ibctest-docker:
	cd ibctest && go test -race -v -run TestRelayerDocker .

ibctest-docker-events:
	cd ibctest && go test -race -v -run TestRelayerDockerEventProcessor .

ibctest-docker-legacy:
	cd ibctest && go test -race -v -run TestRelayerDockerLegacyProcessor .

ibctest-events:
	cd ibctest && go test -race -v -run TestRelayerEventProcessor .

ibctest-legacy:
	cd ibctest && go test -race -v -run TestRelayerLegacyProcessor .

ibctest-multiple:
	cd ibctest && go test -race -v -run TestRelayerMultiplePathsSingleProcess .

ibctest-scenario: ## Scenario tests are suitable for simple networks of 1 validator and no full nodes. They test specific functionality.
	cd ibctest && go test -race -v -run TestScenario .

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
	@./examples/demo/scripts/build-gaia

.PHONY: two-chains test test-integration ibctest install build lint coverage clean
