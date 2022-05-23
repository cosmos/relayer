FROM golang:alpine AS build-env

ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev

RUN apk add --no-cache $PACKAGES

WORKDIR /go/src/github.com/osmosis-labs

RUN wget -O /lib/libwasmvm_muslc.a https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.$(uname -m).a

ARG VERSION

RUN git clone https://github.com/osmosis-labs/osmosis.git

WORKDIR /go/src/github.com/osmosis-labs/osmosis

RUN git checkout ${VERSION} && BUILD_TAGS=muslc make install

FROM alpine:edge

RUN apk add --no-cache ca-certificates gcc
WORKDIR /root

COPY --from=build-env /go/bin/osmosisd /usr/bin/osmosisd

WORKDIR /osmosis

COPY ./_test/setup/osmosis-setup.sh .

COPY ./_test/setup/valkeys ./setup/valkeys

USER root

RUN chmod -R 777 ./setup

EXPOSE 26657

ENTRYPOINT [ "./osmosis-setup.sh" ]
# NOTE: to run this image, docker run -d -p 26657:26657 ./osmosis-setup.sh {{chain_id}} {{genesis_account}} {{seeds}} {{priv_validator_key_path}}