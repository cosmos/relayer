VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
SDKCOMMIT := $(shell go list -m -u -f '{{.Version}}' github.com/cosmos/cosmos-sdk)
# GAIACOMMIT := $(shell go list -m -u -f '{{.Version}}' github.com/cosmos/gaia)
all: ci-lint install

###############################################################################
# Build / Install
###############################################################################

LD_FLAGS = -X github.com/ovrclk/relayer/cmd.Version=$(VERSION) \
	-X github.com/ovrclk/relayer/cmd.Commit=$(COMMIT) \
	-X github.com/ovrclk/relayer/cmd.SDKCommit=$(SDKCOMMIT) \
	-X github.com/ovrclk/relayer/cmd.GaiaCommit=$(GAIACOMMIT)

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
	@go build -mod=readonly $(BUILD_FLAGS) -o $${GOBIN-$${GOPATH-$$HOME/go}/bin}/rly main.go

###############################################################################
# Tests / CI
###############################################################################
test:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/...

test-gaia:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestGaia*

test-mtd:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestMtd*

test-rocketzone:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestRocket*

test-agoric:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestAgoric*

test-coco:
	@TEST_DEBUG=true go test -mod=readonly -v ./test/... -run TestCoCo*

coverage:
	@echo "viewing test coverage..."
	@go tool cover --html=coverage.out

lint:
	@golangci-lint run
	@find . -name '*.go' -type f -not -path "*.git*" | xargs gofmt -d -s
	@go mod verify

.PHONY: install build lint coverage clean

# TODO: Port reproducable build scripts from gaia for relayer
# TODO: Full tested and working releases

###############################################################################
###                           Protobuf                                    ###
###############################################################################

proto-gen:
	./scripts/protocgen.sh

proto-lint:
	buf check lint --error-format=json

proto-check-breaking:
	buf check breaking --against-input '.git#branch=master'

GOGO_PROTO_URL   = https://raw.githubusercontent.com/regen-network/protobuf/cosmos
SDK_IBC_PROTO_URL = https://raw.githubusercontent.com/cosmos/cosmos-sdk/master/proto/ibc

GOGO_PROTO_TYPES    = third_party/proto/gogoproto

SDK_IBC_TYPES  	= third_party/proto/ibc/channel

proto-update-deps:
	mkdir -p $(GOGO_PROTO_TYPES)
	curl -sSL $(GOGO_PROTO_URL)/gogoproto/gogo.proto > $(GOGO_PROTO_TYPES)/gogo.proto

	mkdir -p $(SDK_IBC_TYPES)
	curl -sSL $(SDK_IBC_PROTO_URL)/channel/channel.proto > $(SDK_IBC_TYPES)/channel.proto

PREFIX ?= /usr/local
BIN ?= $(PREFIX)/bin
UNAME_S ?= $(shell uname -s)
UNAME_M ?= $(shell uname -m)

BUF_VERSION ?= 0.11.0

PROTOC_VERSION ?= 3.11.2
ifeq ($(UNAME_S),Linux)
  PROTOC_ZIP ?= protoc-${PROTOC_VERSION}-linux-x86_64.zip
endif
ifeq ($(UNAME_S),Darwin)
  PROTOC_ZIP ?= protoc-${PROTOC_VERSION}-osx-x86_64.zip
endif

proto-tools: proto-tools-stamp buf

proto-tools-stamp:
	echo "Installing protoc compiler..."
	(cd /tmp; \
	curl -OL "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${PROTOC_ZIP}"; \
	unzip -o ${PROTOC_ZIP} -d $(PREFIX) bin/protoc; \
	unzip -o ${PROTOC_ZIP} -d $(PREFIX) 'include/*'; \
	rm -f ${PROTOC_ZIP})

	echo "Installing protoc-gen-gocosmos..."
	go install github.com/regen-network/cosmos-proto/protoc-gen-gocosmos

	# Create dummy file to satisfy dependency and avoid
	# rebuilding when this Makefile target is hit twice
	# in a row
	touch $@

buf: buf-stamp

buf-stamp:
	echo "Installing buf..."
	curl -sSL \
    "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-${UNAME_S}-${UNAME_M}" \
    -o "${BIN}/buf" && \
	chmod +x "${BIN}/buf"

	touch $@

tools-clean:
	rm -f proto-tools-stamp buf-stamp