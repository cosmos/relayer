VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT  := $(shell git log -1 --format='%H')
DIRTY := $(shell git status --porcelain | wc -l | xargs)
GAIA_VERSION := v7.0.1
AKASH_VERSION := v0.16.3
OSMOSIS_VERSION := v8.0.0
WASMD_VERSION := v0.25.0
DOCKER := $(shell which docker)

GOPATH := $(shell go env GOPATH)
GOBIN := $(GOPATH)/bin

all: lint install

###############################################################################
# Build / Install
###############################################################################

ldflags = -X github.com/cosmos/relayer/v2/cmd.Version=$(VERSION) \
					-X github.com/cosmos/relayer/v2/cmd.Commit=$(COMMIT) \
					-X github.com/cosmos/relayer/v2/cmd.Dirty=$(DIRTY)

ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -ldflags '$(ldflags)'

build: go.sum
ifeq ($(OS),Windows_NT)
	@echo "building rly binary..."
	@go build -mod=readonly $(BUILD_FLAGS) -o build/rly.exe main.go
else
	@echo "building rly binary..."
	@go build  $(BUILD_FLAGS) -o build/rly main.go
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

interchaintest:
	cd interchaintest && go test -race -v -run TestRelayerInProcess .

interchaintest-docker:
	cd interchaintest && go test -race -v -run TestRelayerDocker .

interchaintest-docker-events:
	cd interchaintest && go test -race -v -run TestRelayerDockerEventProcessor .

interchaintest-docker-legacy:
	cd interchaintest && go test -race -v -run TestRelayerDockerLegacyProcessor .

interchaintest-events:
	cd interchaintest && go test -race -v -run TestRelayerEventProcessor .

interchaintest-legacy:
	cd interchaintest && go test -race -v -run TestRelayerLegacyProcessor .

interchaintest-multiple:
	cd interchaintest && go test -race -v -run TestRelayerMultiplePathsSingleProcess .

interchaintest-misbehaviour:
	cd interchaintest && go test -race -v -run TestRelayerMisbehaviourDetection .

interchaintest-fee-middleware:
	cd interchaintest && go test -race -v -run TestRelayerFeeMiddleware .

interchaintest-fee-grant:
	cd interchaintest && go test -race -v -run TestRelayerFeeGrant .

interchaintest-scenario: ## Scenario tests are suitable for simple networks of 1 validator and no full nodes. They test specific functionality.
	cd interchaintest && go test -timeout 30m -race -v -run TestScenario ./...

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

CHAIN_CODE := ./chain-code
GAIA_REPO := $(CHAIN_CODE)/gaia

get-gaia:
	@mkdir -p $(CHAIN_CODE)/
	@git clone --branch $(GAIA_VERSION) --depth=1 https://github.com/cosmos/gaia.git $(GAIA_REPO)

build-gaia:
	@[ -d $(GAIA_REPO) ] || { echo "Repositry for gaia does not exist at $(GAIA_REPO). Try running 'make get-gaia'..." ; exit 1; }
	@cd $(GAIA_REPO) && \
	make install &> /dev/null
	@gaiad version --long

.PHONY: two-chains test test-integration interchaintest install build lint coverage clean

PACKAGE_NAME          := github.com/cosmos/relayer
GOLANG_CROSS_VERSION  ?= v1.19.4

SYSROOT_DIR     ?= sysroots
SYSROOT_ARCHIVE ?= sysroots.tar.bz2

.PHONY: sysroot-pack
sysroot-pack:
	@tar cf - $(SYSROOT_DIR) -P | pv -s $[$(du -sk $(SYSROOT_DIR) | awk '{print $1}') * 1024] | pbzip2 > $(SYSROOT_ARCHIVE)

.PHONY: sysroot-unpack
sysroot-unpack:
	@pv $(SYSROOT_ARCHIVE) | pbzip2 -cd | tar -xf -

.PHONY: release-dry-run
release-dry-run:
	@docker run \
		--rm \
		-e CGO_ENABLED=1 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-v `pwd`/sysroot:/sysroot \
		-w /go/src/$(PACKAGE_NAME) \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		--rm-dist --skip-validate --skip-publish

.PHONY: release
release:
	@if [ ! -f ".release-env" ]; then \
		echo "\033[91m.release-env is required for release\033[0m";\
		exit 1;\
	fi
	docker run \
		--rm \
		-e CGO_ENABLED=1 \
		--env-file .release-env \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-v `pwd`/sysroot:/sysroot \
		-w /go/src/$(PACKAGE_NAME) \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --rm-dist

protoVer=0.11.2
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

proto-all: proto-format proto-lint proto-gen

proto-gen:
	@echo "Generating Protobuf files"
	@$(protoImage) sh ./scripts/protocgen.sh

proto-format:
	@$(protoImage) find ./ -name "*.proto" -exec clang-format -i {} \;

proto-lint:
	@$(protoImage) buf lint --error-format=json
