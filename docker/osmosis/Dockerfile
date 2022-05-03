FROM golang:alpine AS build-env
ARG VERSION

ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev

RUN apk add --no-cache $PACKAGES

WORKDIR /go/src/github.com/osmosis-labs

RUN git clone https://github.com/osmosis-labs/osmosis.git

WORKDIR /go/src/github.com/osmosis-labs/osmosis

RUN git checkout ${VERSION} && make build-linux

FROM alpine:edge

RUN apk add --no-cache ca-certificates
WORKDIR /root

COPY --from=build-env /go/src/github.com/osmosis-labs/osmosis/build/osmosisd /usr/bin/osmosisd

WORKDIR /osmosis

COPY ./_test/setup/osmosis-setup.sh .

COPY ./_test/setup/valkeys ./setup/valkeys

USER root

RUN chmod -R 777 ./setup

EXPOSE 26657

ENTRYPOINT [ "./osmosis-setup.sh" ]
# NOTE: to run this image, docker run -d -p 26657:26657 ./osmosis-setup.sh {{chain_id}} {{genesis_account}} {{seeds}} {{priv_validator_key_path}}